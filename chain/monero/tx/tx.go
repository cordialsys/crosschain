package tx

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"filippo.io/edwards25519"
)

// TxInput represents a single input to a Monero transaction
type TxInput struct {
	// Amount (0 for RingCT)
	Amount uint64
	// Key offsets (relative indices of ring members)
	KeyOffsets []uint64
	// Key image (32 bytes)
	KeyImage []byte
}

// TxOutput represents a single output of a Monero transaction
type TxOutput struct {
	// Amount (always 0 for RingCT v2+)
	Amount uint64
	// One-time stealth public key (32 bytes)
	PublicKey []byte
	// View tag (1 byte)
	ViewTag byte
}

// Tx represents a fully constructed Monero transaction.
// For local signing, the CLSAG signatures are computed during Transfer() in the builder.
// The Sighashes()/SetSignatures() interface is preserved but acts as pass-through.
type Tx struct {
	Version    uint8
	UnlockTime uint64
	Inputs     []TxInput
	Outputs    []TxOutput
	Extra      []byte

	// RingCT
	RctType        uint8
	Fee            uint64
	OutCommitments []*edwards25519.Point
	PseudoOuts     []*edwards25519.Point
	EcdhInfo       [][]byte
	BpPlus         *crypto.BulletproofPlus

	// CLSAG signatures (pre-computed by builder for local signing)
	CLSAGs []*crypto.CLSAGSignature

	// Cached serialization
	serialized []byte
}

func (tx *Tx) Hash() xc.TxHash {
	data, err := tx.Serialize()
	if err != nil {
		return ""
	}
	hash := crypto.Keccak256(data)
	return xc.TxHash(hex.EncodeToString(hash))
}

// Sighashes returns empty for Monero since CLSAG is computed in the builder.
// The standard Ed25519 signer cannot produce CLSAG ring signatures.
func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	// Return a dummy sighash - the actual CLSAG signatures are already in tx.CLSAGs
	// This preserves the interface contract (non-empty sighashes for the transfer flow).
	prefixHash := tx.PrefixHash()
	return []*xc.SignatureRequest{
		{Payload: prefixHash},
	}, nil
}

// SetSignatures is a no-op for Monero. CLSAG signatures are set by the builder.
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	// No-op: CLSAGs are already computed
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
		buf = append(buf, 0x02) // txin_to_key
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
		buf = append(buf, 0x03) // txout_to_tagged_key
		buf = append(buf, out.PublicKey...)
		buf = append(buf, out.ViewTag)
	}

	// Extra
	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Extra)))...)
	buf = append(buf, tx.Extra...)

	// RingCT base
	buf = append(buf, tx.RctType)
	if tx.RctType > 0 {
		buf = append(buf, crypto.VarIntEncode(tx.Fee)...)

		// ECDH info
		for _, ecdh := range tx.EcdhInfo {
			buf = append(buf, ecdh...)
		}

		// Output commitments
		for _, c := range tx.OutCommitments {
			buf = append(buf, c.Bytes()...)
		}

		// --- Prunable section ---
		// BP+ proof count + proof
		if tx.BpPlus != nil {
			buf = append(buf, crypto.VarIntEncode(1)...)
			buf = append(buf, tx.BpPlus.Serialize()...)
		}

		// CLSAG signatures
		for _, clsag := range tx.CLSAGs {
			buf = append(buf, clsag.Serialize()...)
		}

		// Pseudo-output commitments
		for _, po := range tx.PseudoOuts {
			buf = append(buf, po.Bytes()...)
		}
	}

	return buf, nil
}

// PrefixHash computes the Keccak256 hash of the transaction prefix.
func (tx *Tx) PrefixHash() []byte {
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

	return crypto.Keccak256(buf)
}
