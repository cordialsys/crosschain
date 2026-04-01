package tx

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"filippo.io/edwards25519"
)

type TxInput struct {
	Amount     uint64
	KeyOffsets []uint64
	KeyImage   []byte // 32 bytes
}

type TxOutput struct {
	Amount    uint64
	PublicKey []byte // 32 bytes
	ViewTag   byte
}

type Tx struct {
	Version    uint8
	UnlockTime uint64
	Inputs     []TxInput
	Outputs    []TxOutput
	Extra      []byte

	// RingCT
	RctType        uint8 // 6 = BulletproofPlus
	Fee            uint64
	OutCommitments []*edwards25519.Point // outPk masks
	PseudoOuts     []*edwards25519.Point
	EcdhInfo       [][]byte // 8 bytes each
	BpPlus         *crypto.BulletproofPlus

	// CLSAG signatures (pre-computed)
	CLSAGs []*crypto.CLSAGSignature

	// Ring size (mixin + 1), needed for CLSAG serialization
	RingSize int
}

func (tx *Tx) Hash() xc.TxHash {
	data, err := tx.Serialize()
	if err != nil {
		return ""
	}
	hash := crypto.Keccak256(data)
	return xc.TxHash(hex.EncodeToString(hash))
}

// Sighashes returns a dummy - CLSAG is computed in the builder.
func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	prefixHash := tx.PrefixHash()
	return []*xc.SignatureRequest{
		{Payload: prefixHash},
	}, nil
}

// SetSignatures is a no-op - CLSAG is pre-computed.
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	return nil
}

// CLSAGMessage computes the three-hash message that CLSAG signs:
// H(prefix_hash || H(rct_sig_base) || H(bp_prunable))
func (tx *Tx) CLSAGMessage() []byte {
	prefixHash := tx.PrefixHash()
	rctBaseHash := crypto.Keccak256(tx.serializeRctBase())
	bpPrunableHash := crypto.Keccak256(tx.serializeBpPrunable())

	// Concatenate as 3 x 32-byte keys, then hash
	combined := make([]byte, 0, 96)
	combined = append(combined, prefixHash...)
	combined = append(combined, rctBaseHash...)
	combined = append(combined, bpPrunableHash...)
	return crypto.Keccak256(combined)
}

// Serialize produces the full transaction in Monero's wire format.
func (tx *Tx) Serialize() ([]byte, error) {
	var buf []byte

	// Transaction prefix
	buf = append(buf, tx.serializePrefix()...)

	// RCT base (inline, not length-prefixed)
	buf = append(buf, tx.serializeRctBase()...)

	// RCT prunable
	buf = append(buf, tx.serializeRctPrunable()...)

	return buf, nil
}

func (tx *Tx) serializePrefix() []byte {
	var buf []byte
	buf = append(buf, crypto.VarIntEncode(uint64(tx.Version))...)
	buf = append(buf, crypto.VarIntEncode(tx.UnlockTime)...)

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

	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Outputs)))...)
	for _, out := range tx.Outputs {
		buf = append(buf, crypto.VarIntEncode(out.Amount)...)
		buf = append(buf, 0x03) // txout_to_tagged_key
		buf = append(buf, out.PublicKey...)
		buf = append(buf, out.ViewTag)
	}

	buf = append(buf, crypto.VarIntEncode(uint64(len(tx.Extra)))...)
	buf = append(buf, tx.Extra...)

	return buf
}

// PrefixHash = Keccak256(serialized prefix)
func (tx *Tx) PrefixHash() []byte {
	return crypto.Keccak256(tx.serializePrefix())
}

// serializeRctBase: type || varint(fee) || ecdhInfo(8 bytes each) || outPk(32 bytes each)
func (tx *Tx) serializeRctBase() []byte {
	var buf []byte
	buf = append(buf, tx.RctType)
	if tx.RctType == 0 {
		return buf
	}

	buf = append(buf, crypto.VarIntEncode(tx.Fee)...)

	// ecdhInfo: 8 bytes per output (truncated amount)
	for _, ecdh := range tx.EcdhInfo {
		if len(ecdh) >= 8 {
			buf = append(buf, ecdh[:8]...)
		} else {
			padded := make([]byte, 8)
			copy(padded, ecdh)
			buf = append(buf, padded...)
		}
	}

	// outPk: 32-byte commitment per output
	for _, c := range tx.OutCommitments {
		buf = append(buf, c.Bytes()...)
	}

	return buf
}

// serializeBpPrunable: the BP+ proof fields as raw keys for hashing.
// This matches get_pre_mlsag_hash's kv construction for RCTTypeBulletproofPlus.
func (tx *Tx) serializeBpPrunable() []byte {
	if tx.BpPlus == nil {
		return nil
	}
	var kv []byte
	bp := tx.BpPlus
	kv = append(kv, bp.A.Bytes()...)
	kv = append(kv, bp.A1.Bytes()...)
	kv = append(kv, bp.B.Bytes()...)
	kv = append(kv, bp.R1.Bytes()...)
	kv = append(kv, bp.S1.Bytes()...)
	kv = append(kv, bp.D1.Bytes()...)
	for _, l := range bp.L {
		kv = append(kv, l.Bytes()...)
	}
	for _, r := range bp.R {
		kv = append(kv, r.Bytes()...)
	}
	return kv
}

// serializeRctPrunable: BP+ proof (with size-prefixed L/R) || CLSAGs || pseudoOuts
func (tx *Tx) serializeRctPrunable() []byte {
	var buf []byte

	// BP+ proof count
	if tx.BpPlus != nil {
		buf = append(buf, crypto.VarIntEncode(1)...) // 1 proof
		bp := tx.BpPlus
		buf = append(buf, bp.A.Bytes()...)
		buf = append(buf, bp.A1.Bytes()...)
		buf = append(buf, bp.B.Bytes()...)
		buf = append(buf, bp.R1.Bytes()...)
		buf = append(buf, bp.S1.Bytes()...)
		buf = append(buf, bp.D1.Bytes()...)
		// L with length prefix
		buf = append(buf, crypto.VarIntEncode(uint64(len(bp.L)))...)
		for _, l := range bp.L {
			buf = append(buf, l.Bytes()...)
		}
		// R with length prefix
		buf = append(buf, crypto.VarIntEncode(uint64(len(bp.R)))...)
		for _, r := range bp.R {
			buf = append(buf, r.Bytes()...)
		}
	}

	// CLSAGs: s[0..ring_size-1] || c1 || D (NO size prefix on s[])
	for _, clsag := range tx.CLSAGs {
		for _, s := range clsag.S {
			buf = append(buf, s.Bytes()...)
		}
		buf = append(buf, clsag.C1.Bytes()...)
		buf = append(buf, clsag.D.Bytes()...)
	}

	// pseudoOuts
	for _, po := range tx.PseudoOuts {
		buf = append(buf, po.Bytes()...)
	}

	return buf
}
