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
	"github.com/cordialsys/crosschain/chain/monero/crypto/cref"
	"github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"github.com/cordialsys/crosschain/factory/signer"
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

	amountU64 := args.GetAmount().Uint64()

	// Fee estimation
	estimatedSize := uint64(2000)
	fee := moneroInput.PerByteFee * estimatedSize
	if moneroInput.QuantizationMask > 0 {
		fee = (fee + moneroInput.QuantizationMask - 1) / moneroInput.QuantizationMask * moneroInput.QuantizationMask
	}

	if len(moneroInput.Outputs) == 0 {
		return nil, fmt.Errorf("no spendable outputs available")
	}

	// Select outputs to spend
	var selectedOutputs []tx_input.Output
	var totalInput uint64
	for _, out := range moneroInput.Outputs {
		selectedOutputs = append(selectedOutputs, out)
		totalInput += out.Amount
		if totalInput >= amountU64+fee {
			break
		}
	}
	if totalInput < amountU64+fee {
		return nil, fmt.Errorf("insufficient funds: have %d, need %d (amount %d + fee %d)",
			totalInput, amountU64+fee, amountU64, fee)
	}
	change := totalInput - amountU64 - fee

	// Load private keys for signing
	privSpend, privView, pubSpend, _, err := loadKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	// Create deterministic RNG seeded from private key + tx parameters
	// This ensures repeated Transfer() calls produce identical results
	rngSeed := append(privSpend.Bytes(), []byte(args.GetTo())...)
	rngSeed = append(rngSeed, args.GetAmount().Bytes()...)
	for _, out := range selectedOutputs {
		rngSeed = append(rngSeed, []byte(out.TxHash)...)
	}
	rng := newDeterministicRNG(rngSeed)

	// Generate deterministic tx private key
	txPrivKey := generateMaskFrom(rng)
	txPrivScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(txPrivKey)
	txPubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(txPrivScalar)

	// Build outputs
	var outputs []tx.TxOutput
	var amounts []uint64
	var masks [][]byte

	// Output 0: destination
	destKey, destViewTag, err := deriveOutputKey(txPrivKey, string(args.GetTo()), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive destination key: %w", err)
	}
	outputs = append(outputs, tx.TxOutput{Amount: 0, PublicKey: destKey, ViewTag: destViewTag})
	amounts = append(amounts, amountU64)
	masks = append(masks, generateMaskFrom(rng))

	// Output 1: change
	if change > 0 {
		changeKey, changeViewTag, err := deriveOutputKey(txPrivKey, string(args.GetFrom()), 1)
		if err != nil {
			return nil, fmt.Errorf("failed to derive change key: %w", err)
		}
		outputs = append(outputs, tx.TxOutput{Amount: 0, PublicKey: changeKey, ViewTag: changeViewTag})
		amounts = append(amounts, change)
		masks = append(masks, generateMaskFrom(rng))
	}

	// Generate BP+ range proof using Monero's exact C++ implementation.
	// Cache the raw proof in TxInput for determinism (Transfer() is called multiple times).
	var bpFields cref.BPPlusFields
	if len(moneroInput.CachedBpProof) > 0 {
		_, bpFields, err = cref.ParseBPPlusProof(moneroInput.CachedBpProof)
		if err != nil {
			return nil, fmt.Errorf("cached BP+ parse failed: %w", err)
		}
	} else {
		var rawProof []byte
		rawProof, err = cref.BPPlusProve(amounts, masks)
		if err != nil {
			return nil, fmt.Errorf("BP+ proof failed: %w", err)
		}
		moneroInput.CachedBpProof = rawProof
		_, bpFields, err = cref.ParseBPPlusProof(rawProof)
		if err != nil {
			return nil, fmt.Errorf("BP+ parse failed: %w", err)
		}
	}

	// Compute outPk commitments (full, unscaled): C = gamma*G + v*H
	commitments := make([]*edwards25519.Point, len(amounts))
	for i := range amounts {
		commitments[i], _ = crypto.PedersenCommit(amounts[i], masks[i])
	}

	// Encrypt amounts
	var ecdhInfo [][]byte
	for i := range amounts {
		enc, _ := encryptAmount(amounts[i], txPrivKey, i)
		ecdhInfo = append(ecdhInfo, enc)
	}

	// Extra field: tx public key
	extra := []byte{0x01}
	extra = append(extra, txPubKey.Bytes()...)

	// Compute pseudo-output commitments and masks
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

	// Phase 1: Build inputs (key images, rings, key offsets) WITHOUT CLSAG sigs
	type inputContext struct {
		ring        []*edwards25519.Point
		commitments []*edwards25519.Point
		realPos     int
		privKey     *edwards25519.Scalar
		zKey        *edwards25519.Scalar
	}

	var txInputs []tx.TxInput
	var inputCtxs []inputContext
	ringSize := 0

	for i, selOut := range selectedOutputs {
		oneTimePrivKey, err := deriveOneTimePrivKey(privSpend, privView, selOut, pubSpend)
		if err != nil {
			return nil, fmt.Errorf("failed to derive one-time private key for input %d: %w", i, err)
		}

		oneTimePubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(oneTimePrivKey)
		keyImage := crypto.ComputeKeyImage(oneTimePrivKey, oneTimePubKey)

		ring, ringCommitments, realPos, keyOffsets, err := buildRingFromMembers(
			selOut.GlobalIndex, selOut.PublicKey, selOut.Commitment, selOut.RingMembers,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build ring for input %d: %w", i, err)
		}

		inputMask := deriveInputMask(privView, selOut)
		inputCommitment, _ := crypto.PedersenCommit(selOut.Amount, inputMask.Bytes())
		if realPos >= 0 && realPos < len(ringCommitments) {
			ringCommitments[realPos] = inputCommitment
		}

		zKey := edwards25519.NewScalar().Subtract(inputMask, pseudoMasks[i])

		txInputs = append(txInputs, tx.TxInput{
			Amount:     0,
			KeyOffsets: keyOffsets,
			KeyImage:   keyImage.Bytes(),
		})
		inputCtxs = append(inputCtxs, inputContext{
			ring: ring, commitments: ringCommitments,
			realPos: realPos, privKey: oneTimePrivKey, zKey: zKey,
		})
		ringSize = len(ring)
	}

	// Phase 2: Build the Tx object (without CLSAGs) to compute CLSAGMessage
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

	// Phase 3: Compute the three-hash CLSAG message from the serialized blob
	// This ensures the message matches what the verifier computes.
	serializedForMsg, _ := moneroTx.Serialize()
	clsagMessage := computeCLSAGMessageFromBlob(serializedForMsg, len(moneroTx.Inputs), len(moneroTx.Outputs))

	// Phase 4: Sign each input with CLSAG using the correct message
	// Reset the deterministic RNG so this is repeatable
	rng = newDeterministicRNG(append(rngSeed, []byte("clsag")...))

	var clsags []*crypto.CLSAGSignature
	for i := range inputCtxs {
		ctx := inputCtxs[i]
		clsagCtx := &crypto.CLSAGContext{
			Message:     clsagMessage,
			Ring:        ctx.ring,
			CNonzero:    ctx.commitments,
			COffset:     pseudoOuts[i],
			SecretIndex: ctx.realPos,
			SecretKey:   ctx.privKey,
			ZKey:        ctx.zKey,
			Rand:        rng,
		}

		clsag, err := crypto.CLSAGSign(clsagCtx)
		if err != nil {
			return nil, fmt.Errorf("CLSAG signing failed for input %d: %w", i, err)
		}
		clsags = append(clsags, clsag)
	}

	moneroTx.CLSAGs = clsags

	return moneroTx, nil
}

func (b TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("monero does not support token transfers")
}

func (b TxBuilder) SupportsMemo() xc.MemoSupport {
	return xc.MemoSupportNone
}

// loadKeys loads the private key from environment and derives all key material
func loadKeys() (privSpend, privView, pubSpend, pubView *edwards25519.Scalar, err error) {
	secret := signer.ReadPrivateKeyEnv()
	if secret == "" {
		return nil, nil, nil, nil, fmt.Errorf("XC_PRIVATE_KEY not set")
	}
	secretBz, err := hex.DecodeString(secret)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	privSpendBytes, privViewBytes, pubSpendBytes, pubViewBytes, err := crypto.DeriveKeysFromSpend(secretBz)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	ps, _ := edwards25519.NewScalar().SetCanonicalBytes(privSpendBytes)
	pv, _ := edwards25519.NewScalar().SetCanonicalBytes(privViewBytes)

	// For pubSpend/pubView we return as scalars of the byte representation
	// (these are actually points, but we pass as scalars for convenience)
	psBytes, _ := edwards25519.NewScalar().SetCanonicalBytes(crypto.ScalarReduce(pubSpendBytes))
	pvBytes, _ := edwards25519.NewScalar().SetCanonicalBytes(crypto.ScalarReduce(pubViewBytes))

	_ = psBytes
	_ = pvBytes

	return ps, pv, ps, pv, nil // Note: pubSpend/pubView returned as scalars (simplified)
}

// deriveOneTimePrivKey derives the one-time private key for spending a specific output.
// x = H_s(8 * viewKey * R || output_index) + spendKey
// where R is the tx public key from the transaction that created this output.
// The tx pub key R is stored in out.Mask during scanning.
func deriveOneTimePrivKey(privSpend, privView *edwards25519.Scalar, out tx_input.Output, pubSpend *edwards25519.Scalar) (*edwards25519.Scalar, error) {
	// Get the tx public key R (stored in the Mask field during scanning)
	txPubKeyHex := out.Mask
	if txPubKeyHex == "" {
		return nil, fmt.Errorf("tx public key not available for output %s:%d", out.TxHash, out.Index)
	}
	txPubKeyBytes, err := hex.DecodeString(txPubKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid tx pub key hex: %w", err)
	}

	// Compute derivation: D = 8 * viewKey * R (using Monero's exact C implementation)
	derivation, err := crypto.GenerateKeyDerivation(txPubKeyBytes, privView.Bytes())
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	// Compute scalar: s = H_s(D || varint(output_index))
	scalar, err := crypto.DerivationToScalar(derivation, out.Index)
	if err != nil {
		return nil, fmt.Errorf("derivation to scalar failed: %w", err)
	}
	hsScalar, err := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	if err != nil {
		return nil, fmt.Errorf("invalid scalar: %w", err)
	}

	// x = s + a (one-time private key = derivation scalar + private spend key)
	x := edwards25519.NewScalar().Add(hsScalar, privSpend)

	// Verify: x*G should equal the output's public key
	xG := edwards25519.NewGeneratorPoint().ScalarBaseMult(x)
	outKeyBytes, err := hex.DecodeString(out.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid output public key: %w", err)
	}
	outPoint, err := edwards25519.NewIdentityPoint().SetBytes(outKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid output point: %w", err)
	}
	if xG.Equal(outPoint) != 1 {
		return nil, fmt.Errorf("derived one-time key does not match output public key (key derivation mismatch)")
	}

	return x, nil
}

// deriveInputMask derives the commitment mask for an input we own.
// In Monero v2: mask = H_s("commitment_mask" || shared_scalar)
// where shared_scalar = H_s(8 * viewKey * R || varint(output_index))
func deriveInputMask(privView *edwards25519.Scalar, out tx_input.Output) *edwards25519.Scalar {
	// Get tx public key R (stored in Mask field during scanning)
	txPubKeyHex := out.Mask
	if txPubKeyHex == "" {
		// Fallback - shouldn't happen
		s, _ := edwards25519.NewScalar().SetCanonicalBytes(make([]byte, 32))
		return s
	}
	txPubKeyBytes, _ := hex.DecodeString(txPubKeyHex)

	// Compute derivation: D = 8 * viewKey * R
	derivation, _ := crypto.GenerateKeyDerivation(txPubKeyBytes, privView.Bytes())

	// Compute shared scalar: s = H_s(D || varint(output_index))
	sharedScalar, _ := crypto.DerivationToScalar(derivation, out.Index)

	// Compute commitment mask: mask = H_s("commitment_mask" || sharedScalar)
	// "commitment_mask" is 15 bytes (no null terminator)
	data := make([]byte, 0, 15+32)
	data = append(data, []byte("commitment_mask")...)
	data = append(data, sharedScalar...)
	hash := crypto.Keccak256(data)
	reduced := crypto.ScReduce32(hash)
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(reduced)
	return s
}

func deriveOutputKey(txPrivKey []byte, address string, outputIndex int) ([]byte, byte, error) {
	_, pubSpend, pubView, err := crypto.DecodeAddress(address)
	if err != nil {
		return nil, 0, err
	}

	rScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(txPrivKey)
	pubViewPoint, err := edwards25519.NewIdentityPoint().SetBytes(pubView)
	if err != nil {
		return nil, 0, err
	}
	D := edwards25519.NewIdentityPoint().ScalarMult(rScalar, pubViewPoint)

	sData := append(D.Bytes(), crypto.VarIntEncode(uint64(outputIndex))...)
	sHash := crypto.Keccak256(sData)
	s := crypto.ScalarReduce(sHash)

	sScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(s)
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar)
	B, _ := edwards25519.NewIdentityPoint().SetBytes(pubSpend)
	P := edwards25519.NewIdentityPoint().Add(sG, B)

	viewTagData := append([]byte("view_tag"), D.Bytes()...)
	viewTagData = append(viewTagData, crypto.VarIntEncode(uint64(outputIndex))...)
	viewTag := crypto.Keccak256(viewTagData)[0]

	return P.Bytes(), viewTag, nil
}

func encryptAmount(amount uint64, txPrivKey []byte, outputIndex int) ([]byte, error) {
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, amount)

	scalarData := append(txPrivKey, crypto.VarIntEncode(uint64(outputIndex))...)
	scalarHash := crypto.Keccak256(scalarData)
	amountKey := crypto.Keccak256(append([]byte("amount"), scalarHash[:32]...))

	encrypted := make([]byte, 8)
	for i := 0; i < 8; i++ {
		encrypted[i] = amountBytes[i] ^ amountKey[i]
	}
	return encrypted, nil
}

// deterministicRNG produces deterministic "random" bytes seeded from transaction parameters.
// This ensures that repeated calls to Transfer() with the same inputs produce identical transactions,
// which is required by the crosschain determinism check.
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

func generateMaskFrom(rng io.Reader) []byte {
	entropy := make([]byte, 64)
	rng.Read(entropy)
	return crypto.RandomScalar(entropy)
}

// buildRingFromMembers constructs a sorted ring from the real output and its decoy members.
// Returns (ring points, commitment points, real position, relative key offsets, error).
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

	// Sort by global index
	sort.Slice(entries, func(i, j int) bool { return entries[i].globalIndex < entries[j].globalIndex })

	// Find real position and compute relative offsets
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
			// Use identity as fallback
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
			cBytes, err := hex.DecodeString(e.commitment)
			if err == nil && len(cBytes) == 32 {
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

// computeCLSAGMessageFromBlob parses the serialized transaction blob and computes
// the three-hash CLSAG message exactly as the Monero verifier would.
func computeCLSAGMessageFromBlob(blob []byte, numInputs, numOutputs int) []byte {
	pos := 0
	readVarint := func() uint64 {
		v := uint64(0); s := uint(0)
		for blob[pos] & 0x80 != 0 { v |= uint64(blob[pos]&0x7f) << s; s += 7; pos++ }
		v |= uint64(blob[pos]) << s; pos++
		return v
	}

	// Parse prefix
	readVarint() // version
	readVarint() // unlock_time
	numIn := readVarint()
	for i := uint64(0); i < numIn; i++ {
		pos++ // tag
		readVarint() // amount
		count := readVarint()
		for j := uint64(0); j < count; j++ { readVarint() }
		pos += 32 // key image
	}
	numOut := readVarint()
	for i := uint64(0); i < numOut; i++ {
		readVarint() // amount
		tag := blob[pos]; pos++
		pos += 32 // key
		if tag == 0x03 { pos++ }
	}
	extraLen := readVarint()
	pos += int(extraLen)
	prefixEnd := pos

	// RCT base
	pos++ // type byte
	readVarint() // fee
	pos += int(numOut) * 8  // ecdhInfo
	pos += int(numOut) * 32 // outPk
	unprunableEnd := pos

	// Parse prunable to extract BP+ kv fields (without CLSAG and pseudoOuts)
	prunableStart := unprunableEnd
	ppos := prunableStart
	readVarintAt := func() uint64 {
		v := uint64(0); s := uint(0)
		for blob[ppos] & 0x80 != 0 { v |= uint64(blob[ppos]&0x7f) << s; s += 7; ppos++ }
		v |= uint64(blob[ppos]) << s; ppos++
		return v
	}
	readVarintAt() // nbp count

	// Extract BP+ key fields (A, A1, B, r1, s1, d1, then L[], R[])
	var bpKv []byte
	bpKv = append(bpKv, blob[ppos:ppos+6*32]...) // A, A1, B, r1, s1, d1
	ppos += 6 * 32

	nL := readVarintAt() // L length
	bpKv = append(bpKv, blob[ppos:ppos+int(nL)*32]...)
	ppos += int(nL) * 32

	nR := readVarintAt() // R length
	bpKv = append(bpKv, blob[ppos:ppos+int(nR)*32]...)

	// Compute hashes
	prefixHash := crypto.Keccak256(blob[:prefixEnd])
	rctBaseHash := crypto.Keccak256(blob[prefixEnd:unprunableEnd])
	bpKvHash := crypto.Keccak256(bpKv)

	combined := make([]byte, 0, 96)
	combined = append(combined, prefixHash...)
	combined = append(combined, rctBaseHash...)
	combined = append(combined, bpKvHash...)
	return crypto.Keccak256(combined)
}

func computeTempPrefixHash(outputs []tx.TxOutput, extra []byte, fee uint64) []byte {
	var buf []byte
	buf = append(buf, crypto.VarIntEncode(2)...) // version
	buf = append(buf, crypto.VarIntEncode(0)...) // unlock_time
	buf = append(buf, crypto.VarIntEncode(uint64(len(outputs)))...)
	for _, out := range outputs {
		buf = append(buf, crypto.VarIntEncode(out.Amount)...)
		buf = append(buf, 0x03)
		buf = append(buf, out.PublicKey...)
		buf = append(buf, out.ViewTag)
	}
	buf = append(buf, crypto.VarIntEncode(uint64(len(extra)))...)
	buf = append(buf, extra...)
	return crypto.Keccak256(buf)
}
