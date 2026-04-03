package builder

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sort"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"filippo.io/edwards25519"
	"golang.org/x/crypto/sha3"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{Asset: cfg}, nil
}

func (b TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return b.NewNativeTransfer(args, input)
}

func (b TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	moneroInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("expected monero TxInput, got %T", input)
	}

	// Get sender's public keys from TransferArgs (no private key access)
	senderPubKey, ok := args.GetPublicKey()
	if !ok || len(senderPubKey) != 64 {
		return nil, fmt.Errorf("sender public key required (64 bytes: pubSpend||pubView)")
	}
	senderPubSpend := senderPubKey[:32]
	senderPubView := senderPubKey[32:]

	amountU64 := args.GetAmount().Uint64()

	// Fee estimation
	estimatedSize := uint64(2000)
	fee := moneroInput.PerByteFee * 200 * estimatedSize / 1024
	if moneroInput.QuantizationMask > 0 {
		fee = (fee + moneroInput.QuantizationMask - 1) / moneroInput.QuantizationMask * moneroInput.QuantizationMask
	}
	if fee < 100000000 {
		fee = 100000000
	}

	if len(moneroInput.Outputs) == 0 {
		return nil, fmt.Errorf("no spendable outputs available")
	}

	// Select outputs (largest first)
	sortedOutputs := make([]tx_input.Output, len(moneroInput.Outputs))
	copy(sortedOutputs, moneroInput.Outputs)
	sort.Slice(sortedOutputs, func(i, j int) bool {
		return sortedOutputs[i].Amount > sortedOutputs[j].Amount
	})

	var selectedOutputs []tx_input.Output
	var totalInput uint64
	for _, out := range sortedOutputs {
		selectedOutputs = append(selectedOutputs, out)
		totalInput += out.Amount
		if totalInput >= amountU64+fee {
			break
		}
	}
	if totalInput < amountU64+fee {
		return nil, fmt.Errorf("insufficient funds: have %d, need %d", totalInput, amountU64+fee)
	}
	change := totalInput - amountU64 - fee

	// Deterministic RNG from TxInput seed
	rngSeed := moneroInput.RngSeed
	if len(rngSeed) == 0 {
		rngSeed = crypto.Keccak256([]byte("default_rng_seed"))
	}
	// Include tx-specific data for uniqueness across repeated calls
	rngSeed = append(rngSeed, []byte(args.GetTo())...)
	rngSeed = append(rngSeed, args.GetAmount().Bytes()...)
	for _, out := range selectedOutputs {
		rngSeed = append(rngSeed, []byte(out.TxHash)...)
	}
	rng := newDeterministicRNG(rngSeed)

	// Generate deterministic tx private key
	txPrivKey := generateMaskFrom(rng)
	txPrivScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(txPrivKey)
	txPubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(txPrivScalar)

	// Build outputs using PUBLIC view keys from addresses (no private keys)
	var outputs []tx.TxOutput
	var amounts []uint64
	var masks [][]byte
	var recipientViews [][]byte

	// Output 0: destination
	_, destPubSpend, destPubView, err := crypto.DecodeAddress(string(args.GetTo()))
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}
	destKey, destViewTag, err := deriveOutputKey(txPrivKey, destPubSpend, destPubView, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive dest key: %w", err)
	}
	outputs = append(outputs, tx.TxOutput{Amount: 0, PublicKey: destKey, ViewTag: destViewTag})
	amounts = append(amounts, amountU64)
	masks = append(masks, deriveOutputMask(txPrivKey, destPubView, 0))
	recipientViews = append(recipientViews, destPubView)

	// Output 1: change (back to sender, using sender's public view key)
	if change > 0 {
		changeKey, changeViewTag, err := deriveOutputKey(txPrivKey, senderPubSpend, senderPubView, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to derive change key: %w", err)
		}
		outputs = append(outputs, tx.TxOutput{Amount: 0, PublicKey: changeKey, ViewTag: changeViewTag})
		amounts = append(amounts, change)
		masks = append(masks, deriveOutputMask(txPrivKey, senderPubView, 1))
		recipientViews = append(recipientViews, senderPubView)
	}

	// BP+ range proof (deterministic from rng)
	var bpFields crypto.BPPlusFields
	if len(moneroInput.CachedBpProof) > 0 {
		_, bpFields, err = crypto.ParseBPPlusProofGo(moneroInput.CachedBpProof)
		if err != nil {
			return nil, fmt.Errorf("cached BP+ parse failed: %w", err)
		}
	} else {
		var rawProof []byte
		rawProof, err = crypto.BPPlusProvePureGo(amounts, masks, rng)
		if err != nil {
			return nil, fmt.Errorf("BP+ proof failed: %w", err)
		}
		moneroInput.CachedBpProof = rawProof
		_, bpFields, err = crypto.ParseBPPlusProofGo(rawProof)
		if err != nil {
			return nil, fmt.Errorf("BP+ parse failed: %w", err)
		}
	}

	// Compute outPk commitments
	commitments := make([]*edwards25519.Point, len(amounts))
	for i := range amounts {
		commitments[i], _ = crypto.PedersenCommit(amounts[i], masks[i])
	}

	// Encrypt amounts
	var ecdhInfo [][]byte
	for i := range amounts {
		enc, _ := encryptAmount(amounts[i], txPrivKey, recipientViews[i], i)
		ecdhInfo = append(ecdhInfo, enc)
	}

	// Extra field: tx public key
	extra := []byte{0x01}
	extra = append(extra, txPubKey.Bytes()...)

	// Compute pseudo-output commitments
	totalOutMask := edwards25519.NewScalar()
	for _, mask := range masks {
		m, _ := edwards25519.NewScalar().SetCanonicalBytes(mask)
		totalOutMask = edwards25519.NewScalar().Add(totalOutMask, m)
	}

	pseudoOuts := make([]*edwards25519.Point, len(selectedOutputs))
	pseudoMasks := make([]*edwards25519.Scalar, len(selectedOutputs))

	if len(selectedOutputs) == 1 {
		pseudoMasks[0], _ = edwards25519.NewScalar().SetCanonicalBytes(totalOutMask.Bytes())
		pseudoOuts[0], _ = crypto.PedersenCommit(totalInput, totalOutMask.Bytes())
	} else {
		runningMask := edwards25519.NewScalar()
		for i := 0; i < len(selectedOutputs)-1; i++ {
			pMask := generateMaskFrom(rng)
			m, _ := edwards25519.NewScalar().SetCanonicalBytes(pMask)
			pseudoMasks[i] = m
			runningMask = edwards25519.NewScalar().Add(runningMask, m)
			pseudoOuts[i], _ = crypto.PedersenCommit(selectedOutputs[i].Amount, pMask)
		}
		lastIdx := len(selectedOutputs) - 1
		lastMask := edwards25519.NewScalar().Subtract(totalOutMask, runningMask)
		pseudoMasks[lastIdx] = lastMask
		pseudoOuts[lastIdx], _ = crypto.PedersenCommit(selectedOutputs[lastIdx].Amount, lastMask.Bytes())
	}

	// Build inputs (key images left empty - computed by signer)
	type clsagInputContext struct {
		Ring           []*edwards25519.Point
		CNonzero       []*edwards25519.Point
		RealPos        int
		KeyOffsets     []uint64
		InputMask      *edwards25519.Scalar // pre-computed commitment mask
		PseudoMask     *edwards25519.Scalar
		OutputKey      string // hex, for signer to derive one-time private key
		TxPubKeyHex    string // hex, original tx pub key
		OutputIndex    uint64
	}
	var txInputs []tx.TxInput
	var clsagContexts []clsagInputContext

	ringSize := 0
	for i, selOut := range selectedOutputs {
		ring, ringCommitments, realPos, keyOffsets, err := buildRingFromMembers(
			selOut.GlobalIndex, selOut.PublicKey, selOut.Commitment, selOut.RingMembers,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build ring for input %d: %w", i, err)
		}

		// Use pre-computed commitment mask from TxInput
		var inputMask *edwards25519.Scalar
		if selOut.CommitmentMask != "" {
			maskBytes, _ := hex.DecodeString(selOut.CommitmentMask)
			inputMask, _ = edwards25519.NewScalar().SetCanonicalBytes(maskBytes)
		} else {
			return nil, fmt.Errorf("input %d missing pre-computed commitment mask", i)
		}

		// Set real output's commitment from our computed mask
		inputCommitment, _ := crypto.PedersenCommit(selOut.Amount, inputMask.Bytes())
		if realPos >= 0 && realPos < len(ringCommitments) {
			ringCommitments[realPos] = inputCommitment
		}

		// Key image placeholder (32 zero bytes - computed by signer)
		keyImage := make([]byte, 32)

		txInputs = append(txInputs, tx.TxInput{
			Amount:     0,
			KeyOffsets: keyOffsets,
			KeyImage:   keyImage,
		})
		clsagContexts = append(clsagContexts, clsagInputContext{
			Ring:        ring,
			CNonzero:    ringCommitments,
			RealPos:     realPos,
			KeyOffsets:  keyOffsets,
			InputMask:   inputMask,
			PseudoMask:  pseudoMasks[i],
			OutputKey:   selOut.PublicKey,
			TxPubKeyHex: selOut.TxPubKey,
			OutputIndex: selOut.Index,
		})
		ringSize = len(ring)
	}

	// Build the unsigned Tx
	moneroTx := &tx.Tx{
		Version:        2,
		UnlockTime:     0,
		Inputs:         txInputs,
		Outputs:        outputs,
		Extra:          extra,
		RctType:        6,
		Fee:            fee,
		OutCommitments: commitments,
		PseudoOuts:     pseudoOuts,
		EcdhInfo:       ecdhInfo,
		BpPlusNative:   &bpFields,
		RingSize:       ringSize,
	}

	// Compute the CLSAG message from the serialized blob
	blobForMsg, _ := moneroTx.Serialize()
	clsagMessage := computeCLSAGMessageFromBlob(blobForMsg, len(txInputs), len(outputs))

	// Store CLSAG contexts on the Tx for the signer to use
	moneroTx.CLSAGContexts = make([]tx.CLSAGInputContext, len(clsagContexts))
	for i, ctx := range clsagContexts {
		// Create per-input RNG seed for deterministic CLSAG nonces
		clsagRngSeed := crypto.Keccak256(append(moneroInput.RngSeed, crypto.VarIntEncode(uint64(i))...))

		moneroTx.CLSAGContexts[i] = tx.CLSAGInputContext{
			Message:     clsagMessage,
			Ring:        ctx.Ring,
			CNonzero:    ctx.CNonzero,
			COffset:     pseudoOuts[i],
			RealPos:     ctx.RealPos,
			InputMask:   ctx.InputMask,
			PseudoMask:  ctx.PseudoMask,
			OutputKey:   ctx.OutputKey,
			TxPubKeyHex: ctx.TxPubKeyHex,
			OutputIndex: ctx.OutputIndex,
			RngSeed:     clsagRngSeed,
		}
	}

	return moneroTx, nil
}

func (b TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("monero does not support token transfers")
}

func (b TxBuilder) SupportsMemo() xc.MemoSupport {
	return xc.MemoSupportNone
}

// deriveOutputKey derives a stealth output key using PUBLIC view key only.
func deriveOutputKey(txPrivKey, pubSpend, pubView []byte, outputIndex int) ([]byte, byte, error) {
	D, err := crypto.GenerateKeyDerivation(pubView, txPrivKey)
	if err != nil {
		return nil, 0, err
	}

	scalar, err := crypto.DerivationToScalar(D, uint64(outputIndex))
	if err != nil {
		return nil, 0, err
	}

	sScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar)
	B, _ := edwards25519.NewIdentityPoint().SetBytes(pubSpend)
	P := edwards25519.NewIdentityPoint().Add(sG, B)

	viewTagData := append([]byte("view_tag"), D...)
	viewTagData = append(viewTagData, crypto.VarIntEncode(uint64(outputIndex))...)
	viewTag := crypto.Keccak256(viewTagData)[0]

	return P.Bytes(), viewTag, nil
}

// deriveOutputMask computes the commitment mask for an output (uses public view key only).
func deriveOutputMask(txPrivKey, recipientPubView []byte, outputIndex int) []byte {
	D, _ := crypto.GenerateKeyDerivation(recipientPubView, txPrivKey)
	scalar, _ := crypto.DerivationToScalar(D, uint64(outputIndex))
	data := make([]byte, 0, 15+32)
	data = append(data, []byte("commitment_mask")...)
	data = append(data, scalar...)
	return crypto.ScReduce32(crypto.Keccak256(data))
}

func encryptAmount(amount uint64, txPrivKey, recipientPubView []byte, outputIndex int) ([]byte, error) {
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, amount)

	D, err := crypto.GenerateKeyDerivation(recipientPubView, txPrivKey)
	if err != nil {
		return nil, err
	}
	scalar, _ := crypto.DerivationToScalar(D, uint64(outputIndex))
	amountKey := crypto.Keccak256(append([]byte("amount"), scalar...))

	encrypted := make([]byte, 8)
	for i := 0; i < 8; i++ {
		encrypted[i] = amountBytes[i] ^ amountKey[i]
	}
	return encrypted, nil
}

func generateMaskFrom(rng io.Reader) []byte {
	entropy := make([]byte, 64)
	rng.Read(entropy)
	return crypto.RandomScalar(entropy)
}

// buildRingFromMembers constructs a sorted ring.
func buildRingFromMembers(
	realGlobalIndex uint64, realKey string, realCommitment string,
	decoys []tx_input.RingMember,
) ([]*edwards25519.Point, []*edwards25519.Point, int, []uint64, error) {
	type ringEntry struct {
		globalIndex uint64
		key         string
		commitment  string
	}

	entries := make([]ringEntry, 0, len(decoys)+1)
	entries = append(entries, ringEntry{realGlobalIndex, realKey, realCommitment})
	for _, d := range decoys {
		entries = append(entries, ringEntry{d.GlobalIndex, d.PublicKey, d.Commitment})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].globalIndex < entries[j].globalIndex })

	realPos := -1
	ring := make([]*edwards25519.Point, len(entries))
	commitments := make([]*edwards25519.Point, len(entries))
	keyOffsets := make([]uint64, len(entries))

	var prevIdx uint64
	for i, e := range entries {
		if e.globalIndex == realGlobalIndex && e.key == realKey {
			realPos = i
		}

		keyBytes, err := hex.DecodeString(e.key)
		if err != nil || len(keyBytes) != 32 {
			ring[i] = edwards25519.NewIdentityPoint()
		} else {
			p, err := edwards25519.NewIdentityPoint().SetBytes(keyBytes)
			if err != nil {
				ring[i] = edwards25519.NewIdentityPoint()
			} else {
				ring[i] = p
			}
		}

		if e.commitment != "" {
			cBytes, _ := hex.DecodeString(e.commitment)
			if len(cBytes) == 32 {
				p, err := edwards25519.NewIdentityPoint().SetBytes(cBytes)
				if err == nil {
					commitments[i] = p
				}
			}
		}
		if commitments[i] == nil {
			commitments[i] = edwards25519.NewIdentityPoint()
		}

		keyOffsets[i] = e.globalIndex - prevIdx
		prevIdx = e.globalIndex
	}

	if realPos < 0 {
		return nil, nil, -1, nil, fmt.Errorf("real output not found in ring")
	}

	return ring, commitments, realPos, keyOffsets, nil
}

// computeCLSAGMessageFromBlob parses the tx blob to compute the CLSAG message.
func computeCLSAGMessageFromBlob(blob []byte, numInputs, numOutputs int) []byte {
	pos := 0
	readVarint := func() uint64 {
		v := uint64(0); s := uint(0)
		for blob[pos] & 0x80 != 0 { v |= uint64(blob[pos]&0x7f) << s; s += 7; pos++ }
		v |= uint64(blob[pos]) << s; pos++
		return v
	}

	readVarint(); readVarint() // version, unlock_time
	numIn := readVarint()
	for i := uint64(0); i < numIn; i++ {
		pos++; readVarint()
		count := readVarint()
		for j := uint64(0); j < count; j++ { readVarint() }
		pos += 32
	}
	numOut := readVarint()
	for i := uint64(0); i < numOut; i++ {
		readVarint(); tag := blob[pos]; pos++; pos += 32
		if tag == 0x03 { pos++ }
	}
	extraLen := readVarint()
	pos += int(extraLen)
	prefixEnd := pos

	pos++; readVarint()
	pos += int(numOut) * 8
	pos += int(numOut) * 32
	rctBaseEnd := pos

	readVarint() // nbp
	kvStart := pos
	pos += 6 * 32
	nL := int(readVarint()); pos += nL * 32
	nR := int(readVarint()); pos += nR * 32

	var kv []byte
	kvPos := kvStart
	kv = append(kv, blob[kvPos:kvPos+6*32]...)
	kvPos += 6 * 32
	for blob[kvPos] & 0x80 != 0 { kvPos++ }; kvPos++
	kv = append(kv, blob[kvPos:kvPos+nL*32]...)
	kvPos += nL * 32
	for blob[kvPos] & 0x80 != 0 { kvPos++ }; kvPos++
	kv = append(kv, blob[kvPos:kvPos+nR*32]...)

	prefixHash := crypto.Keccak256(blob[:prefixEnd])
	rctBaseHash := crypto.Keccak256(blob[prefixEnd:rctBaseEnd])
	bpKvHash := crypto.Keccak256(kv)

	combined := make([]byte, 0, 96)
	combined = append(combined, prefixHash...)
	combined = append(combined, rctBaseHash...)
	combined = append(combined, bpKvHash...)
	return crypto.Keccak256(combined)
}

// --- Deterministic RNG ---

type deterministicRNG struct {
	state []byte
	count uint64
}

func newDeterministicRNG(seed []byte) *deterministicRNG {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("monero_tx_rng"))
	h.Write(seed)
	return &deterministicRNG{state: h.Sum(nil)}
}

func (r *deterministicRNG) Read(p []byte) (int, error) {
	for i := 0; i < len(p); i += 32 {
		h := sha3.NewLegacyKeccak256()
		h.Write(r.state)
		r.count++
		countBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(countBytes, r.count)
		h.Write(countBytes)
		chunk := h.Sum(nil)
		end := i + 32
		if end > len(p) {
			end = len(p)
		}
		copy(p[i:end], chunk[:end-i])
	}
	return len(p), nil
}
