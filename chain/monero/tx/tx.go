package tx

import (
	"encoding/hex"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"filippo.io/edwards25519"
)

// RingMember represents a decoy (or real) output in a ring signature
type RingMember struct {
	// Global output index on the blockchain
	GlobalIndex uint64
	// One-time public key of this output
	PublicKey *edwards25519.Point
	// Pedersen commitment for this output
	Commitment *edwards25519.Point
}

// TxInput represents a single input to a Monero transaction
type TxInput struct {
	// Amount (0 for RingCT, actual amount encoded in commitment)
	Amount uint64
	// Key offsets (relative indices of ring members)
	KeyOffsets []uint64
	// Key image for this input (proves no double-spend)
	KeyImage []byte
	// Ring members (for CLSAG signing)
	Ring []RingMember
	// The index of the real output in the ring
	RealIndex int
}

// TxOutput represents a single output of a Monero transaction
type TxOutput struct {
	// Amount (always 0 for RingCT v2+; real amount is in commitment)
	Amount uint64
	// One-time stealth public key for the recipient
	PublicKey []byte
	// View tag (1 byte, for fast scanning optimization)
	ViewTag byte
}

// Tx represents a Monero transaction under construction.
// The flow is:
//  1. Builder creates the Tx with inputs, outputs, commitments, and BP+ proof
//  2. Sighashes() returns the data needed for CLSAG ring signing
//  3. SetSignatures() attaches the CLSAG ring signatures
//  4. Serialize() produces the final transaction bytes
type Tx struct {
	// Transaction version (2 for RingCT)
	Version uint8
	// Unlock time (0 = no lock)
	UnlockTime uint64

	// Inputs
	Inputs []TxInput
	// Outputs
	Outputs []TxOutput

	// Extra field (contains tx public key, optional payment ID, etc.)
	Extra []byte

	// RingCT data
	RctType uint8 // 6 = CLSAG + Bulletproofs+
	// Transaction fee in atomic units
	Fee uint64
	// Output commitments (Pedersen commitments: amount*H + mask*G)
	OutCommitments []*edwards25519.Point
	// Pseudo-output commitments (for inputs, balance equation)
	PseudoOuts []*edwards25519.Point
	// Encrypted amounts (ecdhInfo)
	EcdhInfo [][]byte
	// Bulletproofs+ range proof
	BpPlus *crypto.BulletproofPlus

	// CLSAG signatures (one per input) - populated by SetSignatures
	CLSAGs [][]byte

	// Transaction prefix hash (for signing)
	prefixHash []byte
}

func (tx *Tx) Hash() xc.TxHash {
	serialized, err := tx.Serialize()
	if err != nil {
		return ""
	}
	hash := crypto.Keccak256(serialized)
	return xc.TxHash(hex.EncodeToString(hash))
}

// Sighashes returns the data that needs to be signed with CLSAG for each input.
// Each SignatureRequest contains:
//   - Payload: the message to sign (transaction prefix hash + RCT hash)
//   - The caller should use this with a CLSAG signer (not standard Ed25519)
func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.Inputs) == 0 {
		return nil, fmt.Errorf("transaction has no inputs")
	}

	// Compute the transaction prefix hash
	prefixHash, err := tx.computePrefixHash()
	if err != nil {
		return nil, fmt.Errorf("failed to compute prefix hash: %w", err)
	}
	tx.prefixHash = prefixHash

	// The CLSAG message is: H(prefix_hash || rct_base_hash || bp_hash)
	// For now, we return the prefix hash as the sighash.
	// The CLSAG signer will need additional context (ring members, key images, etc.)
	// which is available in the TxInput structures.
	requests := make([]*xc.SignatureRequest, len(tx.Inputs))
	for i := range tx.Inputs {
		requests[i] = &xc.SignatureRequest{
			Payload: prefixHash,
		}
	}

	return requests, nil
}

func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(sigs) != len(tx.Inputs) {
		return fmt.Errorf("expected %d signatures (one per input), got %d", len(tx.Inputs), len(sigs))
	}
	tx.CLSAGs = make([][]byte, len(sigs))
	for i, sig := range sigs {
		tx.CLSAGs[i] = sig.Signature
	}
	return nil
}

func (tx *Tx) Serialize() ([]byte, error) {
	var buf []byte

	// Transaction prefix
	buf = append(buf, crypto.VarIntEncode(uint64(tx.Version))...)
	buf = append(buf, crypto.VarIntEncode(tx.UnlockTime)...)

	// Inputs
	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Inputs)))...)
	for _, in := range tx.Inputs {
		buf = append(buf, 0x02) // txin_to_key tag
		buf = append(buf, crypto.VarIntEncode(in.Amount)...)
		buf = append(buf, crypto.VarIntEncode(uint64(len(in.KeyOffsets)))...)
		for _, offset := range in.KeyOffsets {
			buf = append(buf, crypto.VarIntEncode(offset)...)
		}
		buf = append(buf, in.KeyImage...)
	}

	// Outputs
	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Outputs)))...)
	for _, out := range tx.Outputs {
		buf = append(buf, crypto.VarIntEncode(out.Amount)...)
		buf = append(buf, 0x03) // txout_to_tagged_key tag (modern format)
		buf = append(buf, out.PublicKey...)
		buf = append(buf, out.ViewTag)
	}

	// Extra
	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Extra)))...)
	buf = append(buf, tx.Extra...)

	// RingCT
	buf = append(buf, tx.RctType)
	if tx.RctType > 0 {
		// Fee
		buf = append(buf, crypto.VarIntEncode(tx.Fee)...)

		// Pseudo outputs (for CLSAG, these go in the prunable section)
		// In RCT type 6 (CLSAG+BP+), pseudo-outs are in the prunable part

		// ECDH info (encrypted amounts)
		for _, ecdh := range tx.EcdhInfo {
			buf = append(buf, ecdh...)
		}

		// Output commitments
		for _, c := range tx.OutCommitments {
			buf = append(buf, c.Bytes()...)
		}

		// --- Prunable RCT data ---
		// Bulletproofs+
		if tx.BpPlus != nil {
			buf = append(buf, crypto.VarIntEncode(1)...) // number of BP+ proofs
			buf = append(buf, tx.BpPlus.Serialize()...)
		}

		// CLSAG signatures
		for _, clsag := range tx.CLSAGs {
			buf = append(buf, clsag...)
		}

		// Pseudo-output commitments
		for _, po := range tx.PseudoOuts {
			buf = append(buf, po.Bytes()...)
		}
	}

	return buf, nil
}

// computePrefixHash computes the hash of the transaction prefix
// (everything except RCT signatures)
func (tx *Tx) computePrefixHash() ([]byte, error) {
	var buf []byte
	buf = append(buf, crypto.VarIntEncode(uint64(tx.Version))...)
	buf = append(buf, crypto.VarIntEncode(tx.UnlockTime)...)

	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Inputs)))...)
	for _, in := range tx.Inputs {
		buf = append(buf, 0x02)
		buf = append(buf, crypto.VarIntEncode(in.Amount)...)
		buf = append(buf, crypto.VarIntEncode(uint64(len(in.KeyOffsets)))...)
		for _, offset := range in.KeyOffsets {
			buf = append(buf, crypto.VarIntEncode(offset)...)
		}
		buf = append(buf, in.KeyImage...)
	}

	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Outputs)))...)
	for _, out := range tx.Outputs {
		buf = append(buf, crypto.VarIntEncode(out.Amount)...)
		buf = append(buf, 0x03)
		buf = append(buf, out.PublicKey...)
		buf = append(buf, out.ViewTag)
	}

	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Extra)))...)
	buf = append(buf, tx.Extra...)

	return crypto.Keccak256(buf), nil
}
