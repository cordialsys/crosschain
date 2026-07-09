package builder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	"filippo.io/edwards25519"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"github.com/cordialsys/crosschain/pkg/hex"
	"golang.org/x/crypto/sha3"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

// NewTxBuilder returns a Monero tx builder.  The view key is NOT read from
// the chain config; it must be supplied per-transfer via
// builder.OptionViewKey (the CLI plumbs this automatically from the client
// config).
func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{Asset: cfg}, nil
}

// senderPubViewFromArgs derives the sender's public view key from the
// private view key supplied via builder.OptionViewKey.  Returns an error if
// the option is missing or malformed - the tx builder cannot construct a
// stealth-address change output without it.
func senderPubViewFromArgs(args xcbuilder.TransferArgs) ([]byte, error) {
	vkHex, ok := args.GetViewKey()
	if !ok || vkHex == "" {
		return nil, fmt.Errorf("monero tx builder requires a view key (pass builder.OptionViewKey)")
	}
	vk, err := hex.DecodeString(vkHex)
	if err != nil || len(vk) != 32 {
		return nil, fmt.Errorf("monero view key must be 64 hex chars (32 bytes)")
	}
	pubView, err := crypto.PublicFromPrivate(vk)
	if err != nil {
		return nil, fmt.Errorf("invalid monero view key: %w", err)
	}
	return pubView, nil
}

func (b TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return b.NewNativeTransfer(args, input)
}

func (b TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	moneroInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("expected monero TxInput, got %T", input)
	}

	// Get sender's public spend key from TransferArgs (no private key access).
	// The public view key comes from the builder's configured view key.
	senderPubKey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("sender public key required")
	}
	var senderPubSpend []byte
	switch len(senderPubKey) {
	case 32:
		senderPubSpend = senderPubKey
	case 64:
		// Backward-compat: accept (pubSpend||pubView) form; trust our configured view key.
		senderPubSpend = senderPubKey[:32]
	default:
		return nil, fmt.Errorf("sender public key must be 32 bytes (pubSpend), got %d", len(senderPubKey))
	}
	senderPubView, err := senderPubViewFromArgs(args)
	if err != nil {
		return nil, err
	}

	amountU64 := args.GetAmount().Uint64()

	if len(moneroInput.Outputs) == 0 {
		return nil, fmt.Errorf("no spendable outputs available")
	}

	// Like the other utxo chains, the transaction spends every output in the
	// input, returning the excess as change. FetchTransferInput already ran
	// SelectOutputs to allocate the outputs this transfer consumes.
	selectedOutputs := moneroInput.Outputs
	var totalInput uint64
	for _, out := range selectedOutputs {
		totalInput += out.Amount
	}
	fee := moneroInput.EstimatedFeeFor(len(selectedOutputs))
	// Overflow-safe coverage check: fee comes from an untrusted estimate and may
	// be clamped to a very large value, so avoid amountU64+fee wrapping.
	if fee > totalInput || amountU64 > totalInput-fee {
		return nil, fmt.Errorf("insufficient funds: have %d, need %d plus fee %d", totalInput, amountU64, fee)
	}
	change := totalInput - amountU64 - fee

	// Deterministic RNG seed from the TxInput seed plus tx-specific data (for
	// uniqueness across repeated calls). Copied into a fresh slice: appending
	// to the input's slice directly could write into its backing array.
	seed := make([]byte, 0, 128)
	if len(moneroInput.RngSeed) == 0 {
		seed = append(seed, crypto.Keccak256([]byte("default_rng_seed"))...)
	} else {
		seed = append(seed, moneroInput.RngSeed...)
	}
	seed = append(seed, []byte(args.GetTo())...)
	seed = append(seed, args.GetAmount().Bytes()...)
	for _, out := range selectedOutputs {
		seed = append(seed, []byte(out.TxHash)...)
	}
	// Every random value is derived independently from (seed, tag) — never
	// from a shared stream, where a conditional or reordered draw in one
	// component silently shifts every later value (e.g. a build reusing
	// CachedBpProof skips the BP+ draws; on a shared stream the pseudo masks
	// would then come out different than in the build that created the proof).
	rng := newTaggedRNG(seed)

	// Generate deterministic tx private key
	txPrivKey := generateMaskFrom(rng.reader("tx-key"))
	txPrivScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(txPrivKey)
	txPubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(txPrivScalar)

	// Build outputs using PUBLIC view keys from addresses (no private keys).
	// Subaddress handling mirrors wallet2's construct_tx_with_tx_key: classify
	// every destination as standard or subaddress and pick the tx public key(s)
	// accordingly, because a subaddress requires R = r*D (its spend key) rather
	// than R = r*G so the recipient's 8*a*R derivation matches ours.
	destPrefix, destPubSpend, destPubView, err := crypto.DecodeAddress(string(args.GetTo()))
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}
	destIsSub, err := isSubaddressPrefix(destPrefix)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}
	// Guard against cross-network sends (e.g. a mainnet address on a testnet
	// chain): the transfer path builds directly here without going through
	// ValidateAddress, so the check must also live in the builder.
	if err := crypto.CheckAddressNetwork(destPrefix, b.Asset.Network, b.Asset.ChainID.AsString()); err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}

	dests := []txDest{{pubSpend: destPubSpend, pubView: destPubView, isSub: destIsSub, amount: amountU64}}
	// Change (output 1) returns to the sender's own standard address. We append
	// it even when change == 0: an exact transfer (a sweep / inclusive-fee
	// send-max leaves no change) would otherwise be a single-output tx, and
	// Monero consensus since HF12 (HF_VERSION_MIN_2_OUTPUTS) rejects any tx with
	// fewer than 2 outputs at relay. A zero-value output is valid — it still gets
	// a stealth address, view tag, and a commitment to 0 covered by the BP+ range
	// proof — and it keeps sum(outputs) + fee == sum(inputs).
	if change > 0 || len(dests) < 2 {
		dests = append(dests, txDest{pubSpend: senderPubSpend, pubView: senderPubView, isSub: false, amount: change})
	}

	numSub := 0
	for _, d := range dests {
		if d.isSub {
			numSub++
		}
	}
	numStd := len(dests) - numSub
	// Additional per-output tx keys are needed whenever a subaddress is mixed
	// with any other destination (a standard output, or a second subaddress).
	needAdditional := numSub > 0 && (numStd > 0 || numSub > 1)

	// Main tx public key (extra tag 0x01): r*D for a lone subaddress, else r*G.
	mainTxPub := txPubKey
	if numStd == 0 && numSub == 1 {
		dSpend, decErr := edwards25519.NewIdentityPoint().SetBytes(dests[0].pubSpend)
		if decErr != nil {
			return nil, fmt.Errorf("invalid subaddress spend key: %w", decErr)
		}
		mainTxPub = edwards25519.NewIdentityPoint().ScalarMult(txPrivScalar, dSpend)
	}

	// Dedicated deterministic stream for additional tx keys; only drawn on the
	// subaddress path, so standard-only transfers are byte-for-byte unchanged.
	addKeyRng := newDeterministicRNG(append([]byte("addkey:"), seed...))

	var outputs []tx.TxOutput
	var amounts []uint64
	var masks [][]byte
	var recipientViews [][]byte
	var outSecrets [][]byte
	var additionalPubs [][]byte

	for i, d := range dests {
		// Per-output tx secret for the shared-secret derivation. When additional
		// keys are in play, a subaddress output derives from its own key r_i;
		// standard outputs (and the lone-subaddress case) derive from the main r.
		outSecret := txPrivKey
		if needAdditional {
			addSecret := generateMaskFrom(addKeyRng)
			addScalar, decErr := edwards25519.NewScalar().SetCanonicalBytes(addSecret)
			if decErr != nil {
				return nil, fmt.Errorf("invalid additional tx key: %w", decErr)
			}
			var addPub *edwards25519.Point
			if d.isSub {
				outSecret = addSecret
				dSpend, decErr := edwards25519.NewIdentityPoint().SetBytes(d.pubSpend)
				if decErr != nil {
					return nil, fmt.Errorf("invalid subaddress spend key: %w", decErr)
				}
				addPub = edwards25519.NewIdentityPoint().ScalarMult(addScalar, dSpend)
			} else {
				addPub = edwards25519.NewGeneratorPoint().ScalarBaseMult(addScalar)
			}
			additionalPubs = append(additionalPubs, addPub.Bytes())
		}

		outKey, viewTag, derr := deriveOutputKey(outSecret, d.pubSpend, d.pubView, i)
		if derr != nil {
			return nil, fmt.Errorf("failed to derive output %d key: %w", i, derr)
		}
		outputs = append(outputs, tx.TxOutput{Amount: 0, PublicKey: outKey, ViewTag: viewTag})
		amounts = append(amounts, d.amount)
		masks = append(masks, deriveOutputMask(outSecret, d.pubView, i))
		recipientViews = append(recipientViews, d.pubView)
		outSecrets = append(outSecrets, outSecret)
	}

	// BP+ range proof (deterministic from its tagged stream)
	var bpFields crypto.BPPlusFields
	if len(moneroInput.CachedBpProof) > 0 {
		_, bpFields, err = crypto.ParseBPPlusProofGo(moneroInput.CachedBpProof)
		if err != nil {
			return nil, fmt.Errorf("cached BP+ parse failed: %w", err)
		}
	} else {
		var rawProof []byte
		rawProof, err = crypto.BPPlusProvePureGo(amounts, masks, rng.reader("bp+"))
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

	// Encrypt amounts (each with the same per-output secret used for its key)
	var ecdhInfo [][]byte
	for i := range amounts {
		enc, _ := encryptAmount(amounts[i], outSecrets[i], recipientViews[i], i)
		ecdhInfo = append(ecdhInfo, enc)
	}

	// Extra field: main tx public key (tag 0x01), plus per-output additional tx
	// public keys (tag 0x04) when a subaddress is mixed with other destinations.
	extra := []byte{0x01}
	extra = append(extra, mainTxPub.Bytes()...)
	if needAdditional {
		extra = append(extra, 0x04, byte(len(additionalPubs)))
		for _, p := range additionalPubs {
			extra = append(extra, p...)
		}
	}

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
			pMask := generateMaskFrom(rng.reader(fmt.Sprintf("pseudo-mask-%d", i)))
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
	var txInputs []tx.TxInput
	var clsagContexts []tx.CLSAGInputContext
	var spentOutputs []tx.SpentOutputRef

	// Every input must carry the same ring size: Tx.RingSize is a single value
	// used to deserialize all CLSAGs, so a mismatch would misparse them. The size
	// itself is the client's choice (moneroInput.RingSize) — the builder doesn't
	// mandate a specific mixin — and falls back to the first input's size when the
	// client leaves it unset.
	ringSize := moneroInput.RingSize
	for i, selOut := range selectedOutputs {
		ring, ringCommitments, realPos, keyOffsets, err := buildRingFromMembers(
			selOut.GlobalIndex, selOut.PublicKey, selOut.Commitment, selOut.RingMembers,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build ring for input %d: %w", i, err)
		}
		if ringSize == 0 {
			ringSize = len(ring)
		}
		if len(ring) != ringSize {
			return nil, fmt.Errorf("input %d has %d ring members, expected %d (all inputs must share one ring size)", i, len(ring), ringSize)
		}

		// Use pre-computed commitment mask from TxInput. It is untrusted RPC data,
		// so a malformed / non-canonical scalar must error rather than yield a nil
		// mask that panics below.
		if len(selOut.CommitmentMask) == 0 {
			return nil, fmt.Errorf("input %d missing pre-computed commitment mask", i)
		}
		inputMask, err := edwards25519.NewScalar().SetCanonicalBytes(selOut.CommitmentMask)
		if err != nil {
			return nil, fmt.Errorf("input %d has an invalid commitment mask: %w", i, err)
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
		clsagContexts = append(clsagContexts, tx.CLSAGInputContext{
			Ring:        ring,
			CNonzero:    ringCommitments,
			InputMask:   inputMask,
			PseudoMask:  pseudoMasks[i],
			TxPubKeyHex: selOut.TxPubKey.String(),
			OutputIndex: selOut.Index,
		})
		spentOutputs = append(spentOutputs, tx.SpentOutputRef{
			GlobalIndex: selOut.GlobalIndex,
			PublicKey:   selOut.PublicKey.String(),
		})
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
		SpentOutputs:   spentOutputs,
		FromAddress:    string(args.GetFrom()),
	}

	// Compute the CLSAG message from the serialized blob
	blobForMsg, err := moneroTx.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize unsigned tx: %w", err)
	}
	clsagMessage := computeCLSAGMessageFromBlob(blobForMsg, len(txInputs), len(outputs))

	// Store CLSAG contexts on the Tx for the signer to use
	moneroTx.CLSAGContexts = make([]tx.CLSAGInputContext, len(clsagContexts))
	copy(moneroTx.CLSAGContexts, clsagContexts)
	for i := range moneroTx.CLSAGContexts {
		moneroTx.CLSAGContexts[i].Message = clsagMessage
		moneroTx.CLSAGContexts[i].COffset = pseudoOuts[i]
	}

	return moneroTx, nil
}

func (b TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("monero does not support token transfers")
}

func (b TxBuilder) SupportsMemo() xc.MemoSupport {
	return xc.MemoSupportNone
}

// txDest is a resolved transfer destination (standard address or subaddress).
type txDest struct {
	pubSpend []byte
	pubView  []byte
	isSub    bool
	amount   uint64
}

// isSubaddressPrefix reports whether an address prefix denotes a subaddress, and
// rejects prefixes this builder cannot spend to (integrated / unknown).
func isSubaddressPrefix(prefix byte) (bool, error) {
	switch prefix {
	case crypto.MainnetAddressPrefix, crypto.TestnetAddressPrefix:
		return false, nil
	case crypto.MainnetSubaddressPrefix, crypto.TestnetSubaddressPrefix:
		return true, nil
	case crypto.MainnetIntegratedPrefix, crypto.TestnetIntegratedPrefix:
		return false, fmt.Errorf("integrated addresses (payment IDs) are not supported")
	default:
		return false, fmt.Errorf("unsupported monero address prefix: %d", prefix)
	}
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
	B, err := edwards25519.NewIdentityPoint().SetBytes(pubSpend)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid recipient spend key: %w", err)
	}
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
	data = append(data, []byte(crypto.CommitmentMaskLabel)...)
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
	// A short read would silently weaken the mask entropy; the deterministic
	// keccak RNG used here never fails.
	if _, err := io.ReadFull(rng, entropy); err != nil {
		panic(fmt.Sprintf("mask rng failed: %v", err))
	}
	return crypto.RandomScalar(entropy)
}

// buildRingFromMembers constructs a sorted ring from tx_input.RingMember entries.
// See also: client.BuildRing which does the same for client.DecoyOutput types.
func buildRingFromMembers(
	realGlobalIndex uint64, realKey, realCommitment hex.Hex,
	decoys []tx_input.RingMember,
) ([]*edwards25519.Point, []*edwards25519.Point, int, []uint64, error) {
	type ringEntry struct {
		globalIndex uint64
		key         hex.Hex
		commitment  hex.Hex
	}

	entries := make([]ringEntry, 0, len(decoys)+1)
	entries = append(entries, ringEntry{realGlobalIndex, realKey, realCommitment})
	for _, d := range decoys {
		entries = append(entries, ringEntry{d.GlobalIndex, d.PublicKey, d.Commitment})
	}

	// Total order: tie-break equal global indexes by key so sort.Slice (which
	// is unstable) cannot order duplicates differently between builds.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].globalIndex != entries[j].globalIndex {
			return entries[i].globalIndex < entries[j].globalIndex
		}
		return bytes.Compare(entries[i].key, entries[j].key) < 0
	})

	realPos := -1
	ring := make([]*edwards25519.Point, len(entries))
	commitments := make([]*edwards25519.Point, len(entries))
	keyOffsets := make([]uint64, len(entries))

	var prevIdx uint64
	for i, e := range entries {
		if e.globalIndex == realGlobalIndex && bytes.Equal(e.key, realKey) {
			realPos = i
		}

		// A ring member's key or commitment is untrusted RPC data; reject an
		// invalid point instead of substituting the identity, which would sign a
		// tx the daemon is guaranteed to reject.
		p, err := edwards25519.NewIdentityPoint().SetBytes(e.key)
		if err != nil {
			return nil, nil, -1, nil, fmt.Errorf("ring member %d has an invalid public key: %w", i, err)
		}
		ring[i] = p

		if len(e.commitment) > 0 {
			c, err := edwards25519.NewIdentityPoint().SetBytes(e.commitment)
			if err != nil {
				return nil, nil, -1, nil, fmt.Errorf("ring member %d has an invalid commitment: %w", i, err)
			}
			commitments[i] = c
		} else {
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
		v := uint64(0)
		s := uint(0)
		for blob[pos]&0x80 != 0 {
			v |= uint64(blob[pos]&0x7f) << s
			s += 7
			pos++
		}
		v |= uint64(blob[pos]) << s
		pos++
		return v
	}

	readVarint()
	readVarint() // version, unlock_time
	numIn := readVarint()
	for i := uint64(0); i < numIn; i++ {
		pos++
		readVarint()
		count := readVarint()
		for j := uint64(0); j < count; j++ {
			readVarint()
		}
		pos += 32
	}
	numOut := readVarint()
	for i := uint64(0); i < numOut; i++ {
		readVarint()
		tag := blob[pos]
		pos++
		pos += 32
		if tag == 0x03 {
			pos++
		}
	}
	extraLen := readVarint()
	pos += int(extraLen)
	prefixEnd := pos

	pos++
	readVarint()
	pos += int(numOut) * 8
	pos += int(numOut) * 32
	rctBaseEnd := pos

	readVarint() // nbp
	kvStart := pos
	pos += 6 * 32
	nL := int(readVarint())
	pos += nL * 32
	nR := int(readVarint())
	pos += nR * 32

	var kv []byte
	kvPos := kvStart
	kv = append(kv, blob[kvPos:kvPos+6*32]...)
	kvPos += 6 * 32
	for blob[kvPos]&0x80 != 0 {
		kvPos++
	}
	kvPos++
	kv = append(kv, blob[kvPos:kvPos+nL*32]...)
	kvPos += nL * 32
	for blob[kvPos]&0x80 != 0 {
		kvPos++
	}
	kvPos++
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

// taggedRNG derives independent deterministic random streams from one seed.
// Each value depends only on (seed, tag) — not on how many draws happened
// before it — so a conditional or reordered draw in one component can never
// shift the values another component reads. Per-item tags (e.g.
// "pseudo-mask-3") extend the same property across loop iterations.
type taggedRNG struct {
	seed []byte
}

func newTaggedRNG(seed []byte) taggedRNG {
	return taggedRNG{seed: seed}
}

// reader returns the deterministic stream for a tag. The 0x00 separator
// prevents (seed, tag) pairs from colliding across different seed lengths.
func (r taggedRNG) reader(tag string) io.Reader {
	seed := make([]byte, 0, len(r.seed)+1+len(tag))
	seed = append(seed, r.seed...)
	seed = append(seed, 0x00)
	seed = append(seed, tag...)
	return newDeterministicRNG(seed)
}

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
