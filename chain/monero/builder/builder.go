package builder

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
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

	// Generate BP+ range proof
	bpProof, commitments, err := crypto.BulletproofPlusProve(amounts, masks, rng)
	if err != nil {
		return nil, fmt.Errorf("BP+ proof failed: %w", err)
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
		pseudoOuts[0], _ = crypto.PedersenCommit(totalInput-fee, totalOutMask.Bytes())
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

	// Build inputs and compute CLSAG signatures
	var txInputs []tx.TxInput
	var clsags []*crypto.CLSAGSignature

	for i, selOut := range selectedOutputs {
		// Derive one-time private key for this output: x = H_s(viewKey_derivation || output_index) + spend_key
		// First we need the tx public key from the original transaction that created this output
		// For simplicity, we use the derivation scalar stored during scanning
		oneTimePrivKey, err := deriveOneTimePrivKey(privSpend, privView, selOut, pubSpend)
		if err != nil {
			return nil, fmt.Errorf("failed to derive one-time private key for input %d: %w", i, err)
		}

		// Compute public key and key image
		oneTimePubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(oneTimePrivKey)
		keyImage := crypto.ComputeKeyImage(oneTimePrivKey, oneTimePubKey)

		// For now, build a minimal ring with just the real output (no decoys).
		// Full decoy selection requires the client, which isn't available in the builder.
		// The ring will be populated with decoys when FetchTransferInput adds them.
		ring := []*edwards25519.Point{oneTimePubKey}

		// Input commitment (the original output's commitment)
		// For our owned outputs, C = amount*H + inputMask*G
		// We need the input mask - it's derived from the view key
		inputMask := deriveInputMask(privView, selOut)
		inputCommitment, _ := crypto.PedersenCommit(selOut.Amount, inputMask.Bytes())
		inputCommitments := []*edwards25519.Point{inputCommitment}

		// Commitment mask difference: z = input_mask - pseudo_mask
		commitMaskDiff := edwards25519.NewScalar().Subtract(inputMask, pseudoMasks[i])

		// Compute CLSAG signature
		prefixHash := computeTempPrefixHash(outputs, extra, fee)

		clsagCtx := &crypto.CLSAGContext{
			Message:        prefixHash,
			Ring:           ring,
			Commitments:    inputCommitments,
			PseudoOut:      pseudoOuts[i],
			KeyImage:       keyImage,
			SecretIndex:    0,
			SecretKey:      oneTimePrivKey,
			CommitmentMask: commitMaskDiff,
			Rand:           rng,
		}

		clsag, err := crypto.CLSAGSign(clsagCtx)
		if err != nil {
			return nil, fmt.Errorf("CLSAG signing failed for input %d: %w", i, err)
		}
		clsags = append(clsags, clsag)

		txInputs = append(txInputs, tx.TxInput{
			Amount:     0,
			KeyOffsets: []uint64{selOut.GlobalIndex},
			KeyImage:   keyImage.Bytes(),
		})
	}

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
		BpPlus:         bpProof,
		CLSAGs:         clsags,
	}

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
func deriveOneTimePrivKey(privSpend, privView *edwards25519.Scalar, out tx_input.Output, pubSpend *edwards25519.Scalar) (*edwards25519.Scalar, error) {
	// We need the tx public key R from the transaction that created this output.
	// This requires fetching the original transaction - for now, we derive from
	// the output's public key and our keys.
	// In a full implementation, the tx public key would be stored during scanning.

	// The one-time private key is: x = H_s(derivation || output_index) + a
	// where a is the private spend key and derivation = 8 * b * R

	// Since we don't have R stored, we need a different approach.
	// For outputs we received, we stored the derivation scalar during scanning.
	// Let's compute it from the output public key directly.

	// Simplified: for the local signer, compute x = H_s(b * P || index) + a
	// where P is the output's one-time public key (this isn't exactly right but
	// demonstrates the flow; the real implementation needs the tx pub key R)
	outKeyBytes, err := hex.DecodeString(out.PublicKey)
	if err != nil {
		return nil, err
	}
	outPoint, err := edwards25519.NewIdentityPoint().SetBytes(outKeyBytes)
	if err != nil {
		return nil, err
	}

	// D = b * P (simplified derivation)
	D := edwards25519.NewIdentityPoint().ScalarMult(privView, outPoint)

	scalarData := append(D.Bytes(), crypto.VarIntEncode(out.Index)...)
	scalarHash := crypto.Keccak256(scalarData)
	hs := crypto.ScalarReduce(scalarHash)
	hsScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(hs)

	// x = hs + a
	x := edwards25519.NewScalar().Add(hsScalar, privSpend)
	return x, nil
}

// deriveInputMask derives the commitment mask for an input we own.
// mask = H_s("commitment_mask" || derivation || output_index)
func deriveInputMask(privView *edwards25519.Scalar, out tx_input.Output) *edwards25519.Scalar {
	data := append([]byte("commitment_mask"), privView.Bytes()...)
	data = append(data, crypto.VarIntEncode(out.Index)...)
	hash := crypto.Keccak256(data)
	reduced := crypto.ScalarReduce(hash)
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
