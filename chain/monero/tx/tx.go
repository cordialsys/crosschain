package tx

import (
	"encoding/hex"
	"fmt"

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
	BpPlus         *crypto.BulletproofPlus  // Go BP+ (deprecated, kept for compatibility)
	BpPlusNative   *crypto.BPPlusFields      // BP+ proof fields

	// CLSAG signatures (computed by signer via SetSignatures)
	CLSAGs []*crypto.CLSAGSignature

	// CLSAGContexts holds per-input data needed by the signer.
	// Set by the builder, consumed by Sighashes().
	CLSAGContexts []CLSAGInputContext `json:"-"`

	// Ring size (mixin + 1), needed for CLSAG serialization
	RingSize int

	// signingPhase tracks the two-phase signing state
	signingPhase int // 0=unsigned, 1=key images set, 2=fully signed
}

// CLSAGInputContext holds the data the signer needs for one CLSAG ring signature.
type CLSAGInputContext struct {
	Message     []byte                // CLSAG message hash (32 bytes)
	Ring        []*edwards25519.Point // ring member public keys
	CNonzero    []*edwards25519.Point // ring member commitments
	COffset     *edwards25519.Point   // pseudo-output commitment
	RealPos     int                   // position of real output in ring
	InputMask   *edwards25519.Scalar  // pre-computed commitment mask
	PseudoMask  *edwards25519.Scalar  // pseudo-output mask
	OutputKey   string                // hex, output's one-time public key
	TxPubKeyHex string                // hex, original tx public key R
	OutputIndex uint64                // output index in the original tx
	RngSeed     []byte                // for deterministic CLSAG nonces
}

func (tx *Tx) Hash() xc.TxHash {
	// Monero v2 tx hash = Keccak256(prefix_hash || rct_base_hash || rct_prunable_hash)
	prefixHash := tx.PrefixHash()
	rctBaseHash := crypto.Keccak256(tx.SerializeRctBase())
	rctPrunableHash := crypto.Keccak256(tx.SerializeRctPrunable())

	combined := make([]byte, 0, 96)
	combined = append(combined, prefixHash...)
	combined = append(combined, rctBaseHash...)
	combined = append(combined, rctPrunableHash...)

	hash := crypto.Keccak256(combined)
	return xc.TxHash(hex.EncodeToString(hash))
}

// Phase 1: Sighashes returns key-image requests. The signer computes key images
// from the private key and returns them. SetSignatures fills them in, then
// AdditionalSighashes returns the actual CLSAG signing requests.
func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.CLSAGContexts) == 0 {
		// Already signed
		return []*xc.SignatureRequest{{Payload: tx.PrefixHash()}}, nil
	}

	// Phase 1: request key images. Payload = JSON with just enough context
	// for the signer to derive the one-time key and compute the key image.
	requests := make([]*xc.SignatureRequest, len(tx.CLSAGContexts))
	for i, ctx := range tx.CLSAGContexts {
		sh := &MoneroSighash{
			OutputKey:   ctx.OutputKey,
			TxPubKey:    ctx.TxPubKeyHex,
			OutputIndex: ctx.OutputIndex,
		}
		requests[i] = &xc.SignatureRequest{
			Payload: EncodeSighash(sh),
		}
	}
	return requests, nil
}

// SetSignatures handles both phases:
// Phase 1: receives key images (32-byte each). Fills them into tx inputs.
// Phase 2: receives full CLSAG signatures.
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(tx.CLSAGContexts) == 0 {
		return nil
	}

	// Detect phase by signature size
	if len(sigs) > 0 && len(sigs[0].Signature) == 32 {
		// Phase 1: key images (32 bytes each)
		for i, sig := range sigs {
			if i < len(tx.Inputs) {
				tx.Inputs[i].KeyImage = sig.Signature
			}
		}
		tx.signingPhase = 1
		return nil
	}

	// Phase 2: full CLSAG signatures
	if len(sigs) != len(tx.Inputs) {
		return fmt.Errorf("expected %d CLSAG sigs, got %d", len(tx.Inputs), len(sigs))
	}

	tx.CLSAGs = make([]*crypto.CLSAGSignature, len(sigs))
	for i, sig := range sigs {
		clsag, _, err := crypto.DeserializeCLSAG(sig.Signature, tx.RingSize)
		if err != nil {
			return fmt.Errorf("failed to deserialize CLSAG %d: %w", i, err)
		}
		tx.CLSAGs[i] = clsag
	}
	tx.CLSAGContexts = nil
	tx.signingPhase = 2
	return nil
}

// AdditionalSighashes returns the CLSAG signing requests after key images are set.
func (tx *Tx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	if tx.signingPhase != 1 || len(tx.CLSAGContexts) == 0 {
		return nil, nil // no more sighashes needed
	}

	// Now that key images are set, recompute the CLSAG message from the blob
	blob, _ := tx.Serialize()
	clsagMessage := computeCLSAGMessage(blob, len(tx.Inputs), len(tx.Outputs))

	requests := make([]*xc.SignatureRequest, len(tx.CLSAGContexts))
	for i, ctx := range tx.CLSAGContexts {
		ringKeys := make([]string, len(ctx.Ring))
		ringCmts := make([]string, len(ctx.CNonzero))
		for j := range ctx.Ring {
			ringKeys[j] = hex.EncodeToString(ctx.Ring[j].Bytes())
			ringCmts[j] = hex.EncodeToString(ctx.CNonzero[j].Bytes())
		}
		zKey := edwards25519.NewScalar().Subtract(ctx.InputMask, ctx.PseudoMask)

		sh := &MoneroSighash{
			Message:         clsagMessage,
			RingKeys:        ringKeys,
			RingCommitments: ringCmts,
			COffset:         hex.EncodeToString(ctx.COffset.Bytes()),
			RealPos:         ctx.RealPos,
			ZKey:            hex.EncodeToString(zKey.Bytes()),
			OutputKey:       ctx.OutputKey,
			TxPubKey:        ctx.TxPubKeyHex,
			OutputIndex:     ctx.OutputIndex,
			RngSeed:         ctx.RngSeed,
		}
		requests[i] = &xc.SignatureRequest{
			Payload: EncodeSighash(sh),
		}
	}
	return requests, nil
}

// computeCLSAGMessage computes the three-hash CLSAG message from a serialized blob.
func computeCLSAGMessage(blob []byte, numInputs, numOutputs int) []byte {
	pos := 0
	readVarint := func() uint64 {
		v := uint64(0); s := uint(0)
		for blob[pos]&0x80 != 0 { v |= uint64(blob[pos]&0x7f) << s; s += 7; pos++ }
		v |= uint64(blob[pos]) << s; pos++
		return v
	}

	readVarint(); readVarint()
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
	extraLen := readVarint(); pos += int(extraLen)
	prefixEnd := pos

	pos++; readVarint()
	pos += int(numOut)*8 + int(numOut)*32
	rctBaseEnd := pos

	readVarint() // nbp
	kvStart := pos
	pos += 6*32
	nL := int(readVarint()); pos += nL*32
	nR := int(readVarint()); pos += nR*32

	var kv []byte
	kvPos := kvStart
	kv = append(kv, blob[kvPos:kvPos+6*32]...)
	kvPos += 6*32
	for blob[kvPos]&0x80 != 0 { kvPos++ }; kvPos++
	kv = append(kv, blob[kvPos:kvPos+nL*32]...)
	kvPos += nL*32
	for blob[kvPos]&0x80 != 0 { kvPos++ }; kvPos++
	kv = append(kv, blob[kvPos:kvPos+nR*32]...)

	ph := crypto.Keccak256(blob[:prefixEnd])
	bh := crypto.Keccak256(blob[prefixEnd:rctBaseEnd])
	kh := crypto.Keccak256(kv)
	return crypto.Keccak256(append(append(ph, bh...), kh...))
}

// CLSAGMessage computes the three-hash message that CLSAG signs:
// H(prefix_hash || H(rct_sig_base) || H(bp_prunable_kv))
//
// The rct_base hash must match what would be parsed from the serialized blob.
// The bp_prunable hash uses only the BP+ key fields (not CLSAG or pseudoOuts).
func (tx *Tx) CLSAGMessage() []byte {
	// Serialize the full tx to get exact byte boundaries
	prefix := tx.SerializePrefix()
	rctBase := tx.SerializeRctBase()
	bpKv := tx.SerializeBpPrunable() // BP+ key fields only

	prefixHash := crypto.Keccak256(prefix)
	rctBaseHash := crypto.Keccak256(rctBase)
	bpKvHash := crypto.Keccak256(bpKv)

	combined := make([]byte, 0, 96)
	combined = append(combined, prefixHash...)
	combined = append(combined, rctBaseHash...)
	combined = append(combined, bpKvHash...)
	return crypto.Keccak256(combined)
}

// Serialize produces the full transaction in Monero's wire format.
func (tx *Tx) Serialize() ([]byte, error) {
	var buf []byte

	// Transaction prefix
	buf = append(buf, tx.SerializePrefix()...)

	// RCT base (inline, not length-prefixed)
	buf = append(buf, tx.SerializeRctBase()...)

	// RCT prunable
	buf = append(buf, tx.SerializeRctPrunable()...)

	return buf, nil
}

func (tx *Tx) SerializePrefix() []byte {
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
	return crypto.Keccak256(tx.SerializePrefix())
}

// serializeRctBase: type || varint(fee) || ecdhInfo(8 bytes each) || outPk(32 bytes each)
func (tx *Tx) SerializeRctBase() []byte {
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
func (tx *Tx) SerializeBpPrunable() []byte {
	if tx.BpPlusNative != nil {
		var kv []byte
		bp := tx.BpPlusNative
		kv = append(kv, bp.A[:]...)
		kv = append(kv, bp.A1[:]...)
		kv = append(kv, bp.B[:]...)
		kv = append(kv, bp.R1[:]...)
		kv = append(kv, bp.S1[:]...)
		kv = append(kv, bp.D1[:]...)
		for _, l := range bp.L {
			kv = append(kv, l[:]...)
		}
		for _, r := range bp.R {
			kv = append(kv, r[:]...)
		}
		return kv
	}
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
func (tx *Tx) SerializeRctPrunable() []byte {
	var buf []byte

	// BP+ proof
	if tx.BpPlusNative != nil {
		buf = append(buf, crypto.VarIntEncode(1)...) // 1 proof
		bp := tx.BpPlusNative
		buf = append(buf, bp.A[:]...)
		buf = append(buf, bp.A1[:]...)
		buf = append(buf, bp.B[:]...)
		buf = append(buf, bp.R1[:]...)
		buf = append(buf, bp.S1[:]...)
		buf = append(buf, bp.D1[:]...)
		buf = append(buf, crypto.VarIntEncode(uint64(len(bp.L)))...)
		for _, l := range bp.L {
			buf = append(buf, l[:]...)
		}
		buf = append(buf, crypto.VarIntEncode(uint64(len(bp.R)))...)
		for _, r := range bp.R {
			buf = append(buf, r[:]...)
		}
	} else if tx.BpPlus != nil {
		buf = append(buf, crypto.VarIntEncode(1)...)
		bp := tx.BpPlus
		buf = append(buf, bp.A.Bytes()...)
		buf = append(buf, bp.A1.Bytes()...)
		buf = append(buf, bp.B.Bytes()...)
		buf = append(buf, bp.R1.Bytes()...)
		buf = append(buf, bp.S1.Bytes()...)
		buf = append(buf, bp.D1.Bytes()...)
		buf = append(buf, crypto.VarIntEncode(uint64(len(bp.L)))...)
		for _, l := range bp.L {
			buf = append(buf, l.Bytes()...)
		}
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
