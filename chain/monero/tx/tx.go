package tx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"filippo.io/edwards25519"
)

type TxInput struct {
	Amount     uint64
	KeyOffsets []uint64
	KeyImage   []byte // 32 bytes
}

// SpentOutputRef identifies the on-chain output consumed by an input, so that
// after signing (which computes the key image) the client can report the
// authoritative (output, key_image) pair to the light-wallet server. Parallel
// to Tx.Inputs; set by the builder and retained through signing.
type SpentOutputRef struct {
	GlobalIndex uint64 // global output index on the chain (LWS output id)
	PublicKey   string // hex one-time output public key (for debugging/sanity)
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

	// SpentOutputs identifies the consumed output for each input (parallel to
	// Inputs). Unlike CLSAGContexts it is retained after signing so SubmitTx can
	// report the (output, key_image) pairs to the light-wallet server.
	SpentOutputs []SpentOutputRef

	// FromAddress is the sender wallet address. Retained so the (output,
	// key_image) pairs can be imported to the light-wallet server under the
	// correct account.
	FromAddress string

	// Ring size (mixin + 1), needed for CLSAG serialization
	RingSize int

	// signingPhase tracks the two-phase signing state
	signingPhase int // 0=unsigned, 1=key images set, 2=fully signed
}

// CLSAGInputContext holds the data the signer needs for one CLSAG ring
// signature. The real ring position and the output key are intentionally not
// stored here: the signer derives its one-time output key and locates it in the
// ring itself, so neither is placed on the wire.
type CLSAGInputContext struct {
	Message     []byte                // CLSAG message hash (32 bytes)
	Ring        []*edwards25519.Point // ring member public keys
	CNonzero    []*edwards25519.Point // ring member commitments
	COffset     *edwards25519.Point   // pseudo-output commitment
	InputMask   *edwards25519.Scalar  // pre-computed commitment mask
	PseudoMask  *edwards25519.Scalar  // pseudo-output mask
	TxPubKeyHex string                // hex, original tx public key R
	OutputIndex uint64                // output index in the original tx
}

// BroadcastMetadata travels alongside the serialized tx (in
// SubmitTxReq.BroadcastInput) so the client can report the authoritative
// (output, key_image) pairs to the light-wallet server after broadcast.
type BroadcastMetadata struct {
	Sender         string          `json:"sender"`
	SpentKeyImages []SpentKeyImage `json:"spent_key_images"`
}

type SpentKeyImage struct {
	GlobalIndex uint64 `json:"global_index"`
	KeyImage    string `json:"key_image"` // hex
}

// GetMetadata returns the sender address and the (output, key_image) pairs for
// the signed inputs, so SubmitTx can import them to the light-wallet server for
// authoritative spent-output tracking. Returns (nil, false, nil) before signing
// (key images not yet computed) so it is a no-op on unsigned txs.
func (tx *Tx) GetMetadata() ([]byte, bool, error) {
	if len(tx.SpentOutputs) == 0 || len(tx.SpentOutputs) != len(tx.Inputs) {
		return nil, false, nil
	}
	meta := BroadcastMetadata{Sender: tx.FromAddress}
	for i, ref := range tx.SpentOutputs {
		ki := tx.Inputs[i].KeyImage
		if len(ki) != 32 || isZero(ki) {
			// not signed yet
			return nil, false, nil
		}
		meta.SpentKeyImages = append(meta.SpentKeyImages, SpentKeyImage{
			GlobalIndex: ref.GlobalIndex,
			KeyImage:    hex.EncodeToString(ki),
		})
	}
	bz, err := json.Marshal(meta)
	if err != nil {
		return nil, false, fmt.Errorf("failed to encode monero broadcast metadata: %w", err)
	}
	return bz, len(meta.SpentKeyImages) > 0, nil
}

func isZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
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

	// Phase 1: request key images. Payload = the (R, i) the signer needs to
	// derive the one-time key and compute the key image; it derives the output
	// key itself.
	requests := make([]*xc.SignatureRequest, len(tx.CLSAGContexts))
	for i, ctx := range tx.CLSAGContexts {
		sh := &MoneroSighash{
			TxPubKey:    ctx.TxPubKeyHex,
			OutputIndex: ctx.OutputIndex,
		}
		requests[i] = &xc.SignatureRequest{
			Payload: EncodeSighash(sh),
		}
	}
	return requests, nil
}

// SetSignatures applies signer responses for either phase.
//
// The phase is inferred from the number of signatures relative to the number of
// inputs (n), NOT from in-memory state. This matters because the engine rebuilds
// a fresh Tx and replays ALL accumulated signatures on every signing round (see
// engine transaction/serialize.go: GetCrosschainTx re-runs the builder, so the
// unexported signingPhase is 0 and CLSAGContexts is repopulated each round). A
// phase decision keyed on signingPhase would therefore never advance past phase
// 1, and AdditionalSighashes would hand back the phase-2 requests forever.
//
//   - the first n signatures are the phase-1 key images (original input order);
//   - once >= 2n signatures are present, the last n are the phase-2 CLSAGs
//     (ordered to match the sorted contexts AdditionalSighashes issued them from).
//
// Key images are assigned and inputs sorted only once (guarded on whether the
// first key image is already set), so replaying on an already-sorted tx — the
// stateful xc CLI path, which reuses one Tx object — does not reshuffle inputs.
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(tx.CLSAGContexts) == 0 {
		return nil
	}
	n := len(tx.Inputs)
	if n == 0 || len(sigs) < n {
		// Not enough signatures to complete phase 1 yet.
		return nil
	}

	// Phase 1: assign key images and sort, unless already done.
	keyImagesSet := len(tx.Inputs[0].KeyImage) == 32 && !isZero(tx.Inputs[0].KeyImage)
	if !keyImagesSet {
		for i := 0; i < n; i++ {
			tx.Inputs[i].KeyImage = sigs[i].Signature
		}
		tx.sortInputsByKeyImage()
	}

	if len(sigs) < 2*n {
		// Phase 1 complete; phase 2 still pending.
		tx.signingPhase = 1
		return nil
	}

	// Phase 2: the last n signatures are the CLSAGs.
	clsagSigs := sigs[len(sigs)-n:]
	tx.CLSAGs = make([]*crypto.CLSAGSignature, n)
	for i := 0; i < n; i++ {
		clsag, keyImage, err := crypto.DeserializeCLSAG(clsagSigs[i].Signature, tx.RingSize)
		if err != nil {
			return fmt.Errorf("failed to deserialize CLSAG %d: %w", i, err)
		}
		// The signers recompute the key image in phase 2 and embed it in the
		// serialized CLSAG. It must match the phase-1 key image already in the
		// tx prefix (which the CLSAG message was computed over): a mismatch
		// means a poisoned/mismatched phase 1 or misordered responses, and the
		// tx would be rejected on-chain. Catch it before broadcast.
		if !bytes.Equal(keyImage, tx.Inputs[i].KeyImage) {
			return fmt.Errorf(
				"CLSAG %d embedded key image %x does not match the phase-1 key image %x in the tx prefix",
				i, keyImage, tx.Inputs[i].KeyImage,
			)
		}
		tx.CLSAGs[i] = clsag
	}
	tx.signingPhase = 2
	return nil
}

// sortInputsByKeyImage orders inputs by key image, descending. Consensus
// (since hard fork 7) rejects a transaction whose inputs are not sorted this
// way as invalid_input. All per-input parallel slices are permuted together so
// each input keeps its ring context, pseudo-output, and spent-output ref.
// Must run after phase 1 (key images set) and before the CLSAG message is
// computed, since the input order is part of the signed tx prefix.
func (tx *Tx) sortInputsByKeyImage() {
	n := len(tx.Inputs)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		return bytes.Compare(tx.Inputs[order[a]].KeyImage, tx.Inputs[order[b]].KeyImage) > 0
	})

	inputs := make([]TxInput, n)
	for i, j := range order {
		inputs[i] = tx.Inputs[j]
	}
	tx.Inputs = inputs

	if len(tx.CLSAGContexts) == n {
		ctxs := make([]CLSAGInputContext, n)
		for i, j := range order {
			ctxs[i] = tx.CLSAGContexts[j]
		}
		tx.CLSAGContexts = ctxs
	}
	if len(tx.SpentOutputs) == n {
		spent := make([]SpentOutputRef, n)
		for i, j := range order {
			spent[i] = tx.SpentOutputs[j]
		}
		tx.SpentOutputs = spent
	}
	if len(tx.PseudoOuts) == n {
		pseudo := make([]*edwards25519.Point, n)
		for i, j := range order {
			pseudo[i] = tx.PseudoOuts[j]
		}
		tx.PseudoOuts = pseudo
	}
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
			ZKey:            hex.EncodeToString(zKey.Bytes()),
			TxPubKey:        ctx.TxPubKeyHex,
			OutputIndex:     ctx.OutputIndex,
		}
		requests[i] = &xc.SignatureRequest{
			Payload: EncodeSighash(sh),
		}
	}
	return requests, nil
}

// HashFromBlob computes the v2 transaction hash from a serialized tx:
// keccak(prefix_hash || rct_base_hash || rct_prunable_hash). Used by the
// client to check whether a previously-submitted tx is already known to the
// daemon without needing the deserialized Tx.
func HashFromBlob(blob []byte) (xc.TxHash, error) {
	prefixEnd, rctBaseEnd, err := parseTxBoundaries(blob)
	if err != nil {
		return "", err
	}
	combined := make([]byte, 0, 96)
	combined = append(combined, crypto.Keccak256(blob[:prefixEnd])...)
	combined = append(combined, crypto.Keccak256(blob[prefixEnd:rctBaseEnd])...)
	combined = append(combined, crypto.Keccak256(blob[rctBaseEnd:])...)
	return xc.TxHash(hex.EncodeToString(crypto.Keccak256(combined))), nil
}

// parseTxBoundaries walks a serialized v2 transaction and returns the offsets
// where the prefix and the rct base end.
func parseTxBoundaries(blob []byte) (prefixEnd int, rctBaseEnd int, err error) {
	pos := 0
	malformed := fmt.Errorf("malformed tx blob")
	readVarint := func() (uint64, bool) {
		v := uint64(0)
		s := uint(0)
		for {
			if pos >= len(blob) {
				return 0, false
			}
			b := blob[pos]
			pos++
			v |= uint64(b&0x7f) << s
			if b&0x80 == 0 {
				return v, true
			}
			s += 7
		}
	}
	skip := func(n int) bool {
		if pos+n > len(blob) {
			return false
		}
		pos += n
		return true
	}

	if _, ok := readVarint(); !ok { // version
		return 0, 0, malformed
	}
	if _, ok := readVarint(); !ok { // unlock time
		return 0, 0, malformed
	}
	numIn, ok := readVarint()
	if !ok {
		return 0, 0, malformed
	}
	for i := uint64(0); i < numIn; i++ {
		if !skip(1) { // input type
			return 0, 0, malformed
		}
		if _, ok := readVarint(); !ok { // amount
			return 0, 0, malformed
		}
		count, ok := readVarint()
		if !ok {
			return 0, 0, malformed
		}
		for j := uint64(0); j < count; j++ {
			if _, ok := readVarint(); !ok { // key offset
				return 0, 0, malformed
			}
		}
		if !skip(32) { // key image
			return 0, 0, malformed
		}
	}
	numOut, ok := readVarint()
	if !ok {
		return 0, 0, malformed
	}
	for i := uint64(0); i < numOut; i++ {
		if _, ok := readVarint(); !ok { // amount
			return 0, 0, malformed
		}
		if pos >= len(blob) {
			return 0, 0, malformed
		}
		tag := blob[pos]
		pos++
		if !skip(32) { // output key
			return 0, 0, malformed
		}
		if tag == 0x03 && !skip(1) { // view tag
			return 0, 0, malformed
		}
	}
	extraLen, ok := readVarint()
	if !ok || !skip(int(extraLen)) {
		return 0, 0, malformed
	}
	prefixEnd = pos

	if !skip(1) { // rct type
		return 0, 0, malformed
	}
	if _, ok := readVarint(); !ok { // fee
		return 0, 0, malformed
	}
	if !skip(int(numOut)*8 + int(numOut)*32) { // ecdh + outPk
		return 0, 0, malformed
	}
	rctBaseEnd = pos
	if rctBaseEnd >= len(blob) {
		return 0, 0, malformed
	}
	return prefixEnd, rctBaseEnd, nil
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
