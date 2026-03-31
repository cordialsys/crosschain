package builder

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"filippo.io/edwards25519"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfg,
	}, nil
}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewNativeTransfer(args, input)
}

func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	moneroInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("expected monero TxInput, got %T", input)
	}

	amount := args.GetAmount()
	amountU64 := amount.Uint64()

	// Estimate fee (per_byte_fee * estimated_size, quantized)
	estimatedSize := uint64(2000)
	fee := moneroInput.PerByteFee * estimatedSize
	if moneroInput.QuantizationMask > 0 {
		fee = (fee + moneroInput.QuantizationMask - 1) / moneroInput.QuantizationMask * moneroInput.QuantizationMask
	}

	// For now we use the outputs from the TxInput (populated by FetchTransferInput)
	// In a full implementation, these come from scanning with the view key
	if len(moneroInput.Outputs) == 0 {
		// Calculate total needed
		totalNeeded := amountU64 + fee
		return nil, fmt.Errorf("no spendable outputs available (need %d piconero = %d + %d fee)", totalNeeded, amountU64, fee)
	}

	// Select outputs to spend (simple: use all available until we have enough)
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

	// Generate random transaction private key
	txPrivKey := make([]byte, 32)
	rand.Read(txPrivKey)
	txPrivKeyReduced := crypto.ScalarReduce(txPrivKey)

	// Derive tx public key: R = r * G
	txPrivScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(txPrivKeyReduced)
	txPubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(txPrivScalar)

	// Build outputs (destination + change)
	var outputs []tx.TxOutput
	var amounts []uint64
	var masks [][]byte

	// Output 0: destination
	destOutputKey, destViewTag, err := deriveOutputKey(txPrivKeyReduced, string(args.GetTo()), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive destination output key: %w", err)
	}
	outputs = append(outputs, tx.TxOutput{
		Amount:    0, // RingCT: amount hidden in commitment
		PublicKey: destOutputKey,
		ViewTag:   destViewTag,
	})
	amounts = append(amounts, amountU64)
	destMask := generateMask()
	masks = append(masks, destMask)

	// Output 1: change (back to sender)
	if change > 0 {
		changeOutputKey, changeViewTag, err := deriveOutputKey(txPrivKeyReduced, string(args.GetFrom()), 1)
		if err != nil {
			return nil, fmt.Errorf("failed to derive change output key: %w", err)
		}
		outputs = append(outputs, tx.TxOutput{
			Amount:    0,
			PublicKey: changeOutputKey,
			ViewTag:   changeViewTag,
		})
		amounts = append(amounts, change)
		changeMask := generateMask()
		masks = append(masks, changeMask)
	}

	// Generate Bulletproofs+ range proof
	bpProof, commitments, err := crypto.BulletproofPlusProve(amounts, masks)
	if err != nil {
		return nil, fmt.Errorf("bulletproofs+ proof generation failed: %w", err)
	}

	// Encrypt amounts for each output (ecdhInfo)
	var ecdhInfo [][]byte
	for i := range amounts {
		encAmount, err := encryptAmount(amounts[i], txPrivKeyReduced, i)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt amount %d: %w", i, err)
		}
		ecdhInfo = append(ecdhInfo, encAmount)
	}

	// Build extra field: tx public key
	extra := []byte{0x01} // TX_EXTRA_TAG_PUBKEY
	extra = append(extra, txPubKey.Bytes()...)

	// Build inputs with ring members (populated from TxInput)
	var inputs []tx.TxInput
	for _, selOut := range selectedOutputs {
		// Key image placeholder - will be computed by CLSAG signer
		keyImage := make([]byte, 32)

		txIn := tx.TxInput{
			Amount:     0, // RingCT
			KeyOffsets: []uint64{selOut.GlobalIndex}, // Simplified; real impl needs relative offsets with decoys
			KeyImage:   keyImage,
			RealIndex:  0,
		}
		inputs = append(inputs, txIn)
	}

	// Compute pseudo-output commitments (must balance: sum(pseudo) = sum(out_commitments) + fee*H)
	// For simplicity with one input: pseudo_mask = sum(out_masks)
	// With multiple inputs, need to split the masks
	pseudoOuts := make([]*edwards25519.Point, len(inputs))
	if len(inputs) == 1 {
		// Single input: pseudo_mask = sum(output_masks)
		totalMask := edwards25519.NewScalar()
		for _, mask := range masks {
			m, _ := edwards25519.NewScalar().SetCanonicalBytes(mask)
			totalMask = edwards25519.NewScalar().Add(totalMask, m)
		}
		pseudoOuts[0], _ = crypto.PedersenCommit(totalInput-fee, totalMask.Bytes())
	} else {
		// Multiple inputs: split masks across pseudo-outputs
		runningMask := edwards25519.NewScalar()
		for i := 0; i < len(inputs)-1; i++ {
			pMask := generateMask()
			m, _ := edwards25519.NewScalar().SetCanonicalBytes(pMask)
			runningMask = edwards25519.NewScalar().Add(runningMask, m)
			pseudoOuts[i], _ = crypto.PedersenCommit(selectedOutputs[i].Amount, pMask)
		}
		// Last pseudo-out mask must make everything balance
		totalOutMask := edwards25519.NewScalar()
		for _, mask := range masks {
			m, _ := edwards25519.NewScalar().SetCanonicalBytes(mask)
			totalOutMask = edwards25519.NewScalar().Add(totalOutMask, m)
		}
		lastMask := edwards25519.NewScalar().Subtract(totalOutMask, runningMask)
		pseudoOuts[len(inputs)-1], _ = crypto.PedersenCommit(selectedOutputs[len(inputs)-1].Amount, lastMask.Bytes())
	}

	moneroTx := &tx.Tx{
		Version:        2,
		UnlockTime:     0,
		Inputs:         inputs,
		Outputs:        outputs,
		Extra:          extra,
		RctType:        6, // CLSAG + Bulletproofs+
		Fee:            fee,
		OutCommitments: commitments,
		PseudoOuts:     pseudoOuts,
		EcdhInfo:       ecdhInfo,
		BpPlus:         bpProof,
	}

	return moneroTx, nil
}

func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("monero does not support token transfers")
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	return xc.MemoSupportNone
}

// deriveOutputKey derives a one-time stealth output key for a recipient.
// P = H_s(r * pubViewKey || output_index) * G + pubSpendKey
func deriveOutputKey(txPrivKey []byte, address string, outputIndex int) ([]byte, byte, error) {
	_, pubSpend, pubView, err := crypto.DecodeAddress(address)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid address: %w", err)
	}

	// Derivation: D = r * pubViewKey (shared secret)
	rScalar, err := edwards25519.NewScalar().SetCanonicalBytes(txPrivKey)
	if err != nil {
		return nil, 0, err
	}
	pubViewPoint, err := edwards25519.NewIdentityPoint().SetBytes(pubView)
	if err != nil {
		return nil, 0, err
	}
	D := edwards25519.NewIdentityPoint().ScalarMult(rScalar, pubViewPoint)

	// s = H_s(D || output_index)
	sData := append(D.Bytes(), crypto.VarIntEncode(uint64(outputIndex))...)
	sHash := crypto.Keccak256(sData)
	s := crypto.ScalarReduce(sHash)

	// P = s * G + pubSpendKey
	sScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(s)
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar)
	B, _ := edwards25519.NewIdentityPoint().SetBytes(pubSpend)
	P := edwards25519.NewIdentityPoint().Add(sG, B)

	// View tag = first byte of H("view_tag" || D || output_index)
	viewTagData := append([]byte("view_tag"), D.Bytes()...)
	viewTagData = append(viewTagData, crypto.VarIntEncode(uint64(outputIndex))...)
	viewTag := crypto.Keccak256(viewTagData)[0]

	return P.Bytes(), viewTag, nil
}

// encryptAmount encrypts an output amount for the recipient using ECDH.
// encrypted = amount XOR first_8_bytes(H_s("amount" || shared_scalar))
func encryptAmount(amount uint64, txPrivKey []byte, outputIndex int) ([]byte, error) {
	// For now, produce the 8-byte encrypted amount
	// The recipient can decrypt using their view key
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, amount)

	// Simplified: in a real implementation, the shared scalar is derived
	// from the ECDH shared secret between tx private key and recipient's view key.
	// For now, we XOR with a deterministic value derived from tx key and output index.
	scalarData := append(txPrivKey, crypto.VarIntEncode(uint64(outputIndex))...)
	scalarHash := crypto.Keccak256(scalarData)
	amountKey := crypto.Keccak256(append([]byte("amount"), scalarHash[:32]...))

	encrypted := make([]byte, 8)
	for i := 0; i < 8; i++ {
		encrypted[i] = amountBytes[i] ^ amountKey[i]
	}
	return encrypted, nil
}

func generateMask() []byte {
	entropy := make([]byte, 64)
	rand.Read(entropy)
	return crypto.RandomScalar(entropy)
}
