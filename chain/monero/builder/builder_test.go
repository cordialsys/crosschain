package builder_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"filippo.io/edwards25519"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	monerotx "github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"github.com/cordialsys/crosschain/pkg/hex"
	"github.com/stretchr/testify/require"
)

func mustHex(t *testing.T, s string) hex.Hex {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

const (
	testFromAddress = "A1QNoLe4UHyVg7v9EPWLMoMP2XRqPytg7DXjSqJhSJd637MXtRKEqn1hpjTJodDj66RVewXjw6NAzN2px5QbgqsvFJSoRYf"
	testToAddress   = "9wppZCoZtBD47RB3kY3i5HfYVqMgFXyEx368ts1ugHBHhFEu4U1hUmwhpjTJodDj66RVewXjw6NAzN2px5QbgqsvFFDMhXW"
	// Any canonical scalar works: the builder only derives the public view key
	// from it for the change output.
	testViewKey = "0f00000000000000000000000000000000000000000000000000000000000000"
	// Valid curve points for synthetic ring member data.
	pointG = "5866666666666666666666666666666666666666666666666666666666666666"
	pointH = "8b655970153799af2aeadc9ff1add0ea6c7251d54154cfa92c173a0dd39c1f94"
)

func makeTransferInput(t *testing.T) *tx_input.TxInput {
	t.Helper()
	input := tx_input.NewTxInput()
	input.BlockHeight = 3000000
	input.PerByteFee = 20000
	input.QuantizationMask = 10000
	input.RngSeed = crypto.Keccak256([]byte("test_seed"))

	pg, ph, vk := mustHex(t, pointG), mustHex(t, pointH), mustHex(t, testViewKey)
	// Two spendable outputs so the tx has multiple inputs (multiple pseudo
	// masks) and produces change.
	for i := 0; i < 2; i++ {
		out := tx_input.Output{
			Amount:         10_000_000_000,
			Index:          0,
			TxHash:         fmt.Sprintf("tx%d", i),
			GlobalIndex:    uint64(7000000 + i*100),
			PublicKey:      pg,
			Commitment:     ph,
			TxPubKey:       pg,
			CommitmentMask: vk,
		}
		for j := 0; j < 15; j++ {
			out.RingMembers = append(out.RingMembers, tx_input.RingMember{
				GlobalIndex: uint64(1000000 + i*100000 + j*1000),
				PublicKey:   pg,
				Commitment:  ph,
			})
		}
		input.Outputs = append(input.Outputs, out)
	}
	return input
}

func makeTransferArgs(t *testing.T) xcbuilder.TransferArgs {
	t.Helper()
	_, pubSpend, _, err := crypto.DecodeAddress(testFromAddress)
	require.NoError(t, err)
	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero},
		xc.Address(testFromAddress),
		xc.Address(testToAddress),
		xc.NewAmountBlockchainFromUint64(5_000_000_000),
		xcbuilder.OptionPublicKey(pubSpend),
		xcbuilder.OptionViewKey(testViewKey),
	)
	require.NoError(t, err)
	return args
}

// txFingerprint captures every builder-determined byte of an unsigned tx.
func txFingerprint(t *testing.T, ixc xc.Tx) []byte {
	t.Helper()
	mtx, ok := ixc.(*monerotx.Tx)
	require.True(t, ok)

	var fp []byte
	fp = append(fp, mtx.PrefixHash()...)
	fp = append(fp, mtx.SerializeRctBase()...)
	fp = append(fp, mtx.SerializeBpPrunable()...)
	for _, ps := range mtx.PseudoOuts {
		fp = append(fp, ps.Bytes()...)
	}
	for _, ctx := range mtx.CLSAGContexts {
		fp = append(fp, ctx.InputMask.Bytes()...)
		fp = append(fp, ctx.PseudoMask.Bytes()...)
	}
	return fp
}

// TestTransferDeterministic requires that building the same transfer twice
// yields a byte-identical transaction — including the second build, which
// takes the CachedBpProof path and so skips the BP+ RNG draws. Any
// non-determinism in the builder (shared RNG stream offsets, map iteration,
// unstable sorts) breaks the engine's rebuild-and-replay signing flow.
func TestTransferDeterministic(t *testing.T) {
	b, err := builder.NewTxBuilder(&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero})
	require.NoError(t, err)
	args := makeTransferArgs(t)

	// Build once from a fresh input (proves and caches the BP+ proof).
	input := makeTransferInput(t)
	tx1, err := b.Transfer(args, input)
	require.NoError(t, err)
	require.NotEmpty(t, input.CachedBpProof, "first build caches the BP+ proof")

	// Rebuild with the same input object: cached BP+ path.
	tx2, err := b.Transfer(args, input)
	require.NoError(t, err)

	// Rebuild from a fresh input: uncached BP+ path.
	tx3, err := b.Transfer(args, makeTransferInput(t))
	require.NoError(t, err)

	fp1 := txFingerprint(t, tx1)
	require.Equal(t, fp1, txFingerprint(t, tx2), "cached rebuild must be byte-identical")
	require.Equal(t, fp1, txFingerprint(t, tx3), "fresh rebuild must be byte-identical")

	require.Equal(t, hex.EncodeToString(input.CachedBpProof),
		hex.EncodeToString(makeCachedProof(t, b, args)),
		"BP+ proof itself must be deterministic")
}

// TestTransferExactAmountAddsDummyChange is the M2 regression test: an exact
// transfer that leaves no change (e.g. a sweep) must still produce >= 2 outputs,
// since Monero consensus rejects single-output txs. The appended output goes to
// the sender and is zero-value.
func TestTransferExactAmountAddsDummyChange(t *testing.T) {
	b, err := builder.NewTxBuilder(&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero})
	require.NoError(t, err)

	input := makeTransferInput(t)
	var totalInput uint64
	for _, o := range input.Outputs {
		totalInput += o.Amount
	}
	// Amount chosen so change == totalInput - amount - fee == 0.
	exact := totalInput - input.EstimatedFeeFor(len(input.Outputs))

	_, senderPub, _, err := crypto.DecodeAddress(testFromAddress)
	require.NoError(t, err)
	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero},
		xc.Address(testFromAddress), xc.Address(testToAddress),
		xc.NewAmountBlockchainFromUint64(exact),
		xcbuilder.OptionPublicKey(senderPub), xcbuilder.OptionViewKey(testViewKey),
	)
	require.NoError(t, err)

	itx, err := b.Transfer(args, input)
	require.NoError(t, err)
	mtx := itx.(*monerotx.Tx)

	require.Len(t, mtx.Outputs, 2, "exact-amount transfer must still emit a 2nd output")

	// Output 1 is the appended change: it belongs to the sender and is 0-value.
	mainR, _ := parseExtra(t, mtx.Extra)
	viewBytes, err := hex.DecodeString(testViewKey)
	require.NoError(t, err)
	matched, _, amount, err := crypto.ScanOutputForSubaddresses(
		mainR, 1, hex.EncodeToString(mtx.Outputs[1].PublicKey),
		hex.EncodeToString(mtx.EcdhInfo[1]), viewBytes, senderPub, nil)
	require.NoError(t, err)
	require.True(t, matched, "appended output should belong to the sender")
	require.EqualValues(t, 0, amount, "appended output must be zero-value")
}

// TestTransferRingSizeConsistency covers M3: all inputs must share one ring
// size (so the single Tx.RingSize deserializes every CLSAG), the size is taken
// from the TxInput when the client sets it, and the builder does not mandate a
// specific mixin.
func TestTransferRingSizeConsistency(t *testing.T) {
	b, err := builder.NewTxBuilder(&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero})
	require.NoError(t, err)
	args := makeTransferArgs(t)

	// Heterogeneous ring sizes across inputs are rejected.
	hetero := makeTransferInput(t)
	hetero.Outputs[1].RingMembers = hetero.Outputs[1].RingMembers[:10]
	_, err = b.Transfer(args, hetero)
	require.ErrorContains(t, err, "ring members")

	// An explicit RingSize the inputs don't match is rejected (client-declared).
	declared := makeTransferInput(t)
	declared.RingSize = 21 // outputs carry 16 members
	_, err = b.Transfer(args, declared)
	require.ErrorContains(t, err, "expected 21")

	// A non-default but consistent ring size the client asked for is accepted:
	// the builder enforces consistency, not a hardcoded 16.
	custom := makeTransferInput(t)
	for i := range custom.Outputs {
		custom.Outputs[i].RingMembers = custom.Outputs[i].RingMembers[:10] // 11 members
	}
	custom.RingSize = 11
	_, err = b.Transfer(args, custom)
	require.NoError(t, err)
}

// TestTransferRejectsMalformedInputData covers M6: malformed hex/point/scalar
// data in an (untrusted) TxInput must return an error instead of panicking.
func TestTransferRejectsMalformedInputData(t *testing.T) {
	b, err := builder.NewTxBuilder(&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero})
	require.NoError(t, err)
	args := makeTransferArgs(t)

	// Wrong-length commitment mask -> clean error, not a nil-scalar panic.
	badMask := makeTransferInput(t)
	badMask.Outputs[0].CommitmentMask = make([]byte, 2)
	_, err = b.Transfer(args, badMask)
	require.ErrorContains(t, err, "commitment mask")

	// Wrong-length ring member key -> clean error, not a nil-point panic.
	badRing := makeTransferInput(t)
	badRing.Outputs[0].RingMembers[0].PublicKey = make([]byte, 31)
	_, err = b.Transfer(args, badRing)
	require.ErrorContains(t, err, "invalid public key")
}

func makeCachedProof(t *testing.T, b builder.TxBuilder, args xcbuilder.TransferArgs) []byte {
	t.Helper()
	input := makeTransferInput(t)
	_, err := b.Transfer(args, input)
	require.NoError(t, err)
	return input.CachedBpProof
}

// parseExtra pulls the main tx public key (tag 0x01) and the per-output
// additional tx public keys (tag 0x04) out of a tx extra field.
func parseExtra(t *testing.T, extra []byte) (mainR []byte, additional [][]byte) {
	t.Helper()
	for i := 0; i < len(extra); {
		tag := extra[i]
		i++
		switch tag {
		case 0x01:
			require.LessOrEqual(t, i+32, len(extra))
			mainR = extra[i : i+32]
			i += 32
		case 0x04:
			require.Less(t, i, len(extra))
			count := int(extra[i])
			i++
			for k := 0; k < count; k++ {
				require.LessOrEqual(t, i+32, len(extra))
				additional = append(additional, extra[i:i+32])
				i += 32
			}
		default:
			t.Fatalf("unexpected extra tag 0x%02x", tag)
		}
	}
	return mainR, additional
}

// subaddressSecretSpendKey recomputes the recipient's private spend key for a
// subaddress: d = b + H_s("SubAddr\0" || a || major || minor), the same scalar
// crypto.DeriveSubaddressKeys folds into the public spend key.
func subaddressSecretSpendKey(t *testing.T, privView, privSpend []byte, idx crypto.SubaddressIndex) *edwards25519.Scalar {
	t.Helper()
	data := append([]byte("SubAddr\x00"), privView...)
	major := make([]byte, 4)
	binary.LittleEndian.PutUint32(major, idx.Major)
	minor := make([]byte, 4)
	binary.LittleEndian.PutUint32(minor, idx.Minor)
	data = append(data, major...)
	data = append(data, minor...)
	m, err := edwards25519.NewScalar().SetCanonicalBytes(crypto.ScalarReduce(crypto.Keccak256(data)))
	require.NoError(t, err)
	b, err := edwards25519.NewScalar().SetCanonicalBytes(privSpend)
	require.NoError(t, err)
	return edwards25519.NewScalar().Add(b, m)
}

// TestTransferToSubaddress is the regression test for the fund-burning bug:
// a transfer to a subaddress must produce an output the recipient can both
// detect (via the per-output additional tx key) and spend. Building the output
// with the main R=r*G key — as the old builder did — leaves it undetectable.
func TestTransferToSubaddress(t *testing.T) {
	// Recipient wallet we fully control, so we can scan and check spendability.
	privView := crypto.ScalarReduce(crypto.Keccak256([]byte("recipient-view")))
	privSpend := crypto.ScalarReduce(crypto.Keccak256([]byte("recipient-spend")))
	pubSpend, err := crypto.PublicFromPrivate(privSpend)
	require.NoError(t, err)

	idx := crypto.SubaddressIndex{Major: 0, Minor: 1}
	subSpendPub, subViewPub, err := crypto.DeriveSubaddressKeys(privView, pubSpend, idx)
	require.NoError(t, err)
	subAddr := crypto.GenerateAddressWithPrefix(crypto.MainnetSubaddressPrefix, subSpendPub, subViewPub)

	// Sanity: our recomputed secret spend key matches the public spend key.
	dSecret := subaddressSecretSpendKey(t, privView, privSpend, idx)
	require.Equal(t, subSpendPub, edwards25519.NewGeneratorPoint().ScalarBaseMult(dSecret).Bytes())

	const sendAmount = uint64(5_000_000_000)
	_, senderPub, _, err := crypto.DecodeAddress(testFromAddress)
	require.NoError(t, err)

	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero},
		xc.Address(testFromAddress),
		xc.Address(subAddr),
		xc.NewAmountBlockchainFromUint64(sendAmount),
		xcbuilder.OptionPublicKey(senderPub),
		xcbuilder.OptionViewKey(testViewKey),
	)
	require.NoError(t, err)

	b, err := builder.NewTxBuilder(&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero})
	require.NoError(t, err)
	itx, err := b.Transfer(args, makeTransferInput(t))
	require.NoError(t, err)
	mtx := itx.(*monerotx.Tx)

	// Output 0 is the subaddress destination; output 1 is change to the sender.
	require.Len(t, mtx.Outputs, 2)
	mainR, additional := parseExtra(t, mtx.Extra)
	require.Len(t, additional, 2, "mixed subaddress+standard tx must carry one additional key per output")

	out0 := hex.EncodeToString(mtx.Outputs[0].PublicKey)
	ecdh0 := hex.EncodeToString(mtx.EcdhInfo[0])
	subKeys := map[crypto.SubaddressIndex][]byte{idx: subSpendPub}

	// The recipient detects the output using the additional key for output 0.
	matched, matchedIdx, amount, err := crypto.ScanOutputForSubaddresses(
		additional[0], 0, out0, ecdh0, privView, pubSpend, subKeys)
	require.NoError(t, err)
	require.True(t, matched, "subaddress output must be detectable via its additional tx key")
	require.Equal(t, idx, matchedIdx)
	require.Equal(t, sendAmount, amount)

	// And the naive main-R scan (the old buggy behavior) must NOT match — this
	// is exactly why the old builder burned funds.
	matchedMain, _, _, err := crypto.ScanOutputForSubaddresses(
		mainR, 0, out0, ecdh0, privView, pubSpend, subKeys)
	require.NoError(t, err)
	require.False(t, matchedMain, "main tx key must not derive the subaddress output")

	// Spendability: the one-time secret x = H_s(8*a*R_add || 0) + d must open the
	// output public key. If it does, the recipient can sign a CLSAG for it.
	derivation, err := crypto.GenerateKeyDerivation(additional[0], privView)
	require.NoError(t, err)
	sBytes, err := crypto.DerivationToScalar(derivation, 0)
	require.NoError(t, err)
	s, err := edwards25519.NewScalar().SetCanonicalBytes(sBytes)
	require.NoError(t, err)
	x := edwards25519.NewScalar().Add(s, dSecret)
	require.Equal(t, mtx.Outputs[0].PublicKey,
		edwards25519.NewGeneratorPoint().ScalarBaseMult(x).Bytes(),
		"recovered one-time key must equal the output public key")
}

// TestTransferIntegratedAddressRejected ensures unsupported address types fail
// loudly at build time rather than silently burning funds.
func TestTransferRejectsUnsupportedPrefix(t *testing.T) {
	b, err := builder.NewTxBuilder(&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero})
	require.NoError(t, err)
	_, senderPub, _, err := crypto.DecodeAddress(testFromAddress)
	require.NoError(t, err)
	// A well-formed address with an unknown prefix byte (99).
	_, pubSpend, pubView, err := crypto.DecodeAddress(testToAddress)
	require.NoError(t, err)
	bad := crypto.GenerateAddressWithPrefix(99, pubSpend, pubView)
	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero},
		xc.Address(testFromAddress), xc.Address(bad),
		xc.NewAmountBlockchainFromUint64(5_000_000_000),
		xcbuilder.OptionPublicKey(senderPub), xcbuilder.OptionViewKey(testViewKey),
	)
	require.NoError(t, err)
	_, err = b.Transfer(args, makeTransferInput(t))
	require.ErrorContains(t, err, "unsupported monero address prefix")
}

// TestTransferRejectsCrossNetwork guards the exact mistake of sending a testnet
// transfer to a mainnet address: a mainnet-prefixed destination on a
// testnet-configured builder (and the reverse) must fail before building.
func TestTransferRejectsCrossNetwork(t *testing.T) {
	// testFromAddress / testToAddress are testnet (prefix 53). Build a mainnet
	// standard address from the same keys for the mismatch case.
	_, toSpend, toView, err := crypto.DecodeAddress(testToAddress)
	require.NoError(t, err)
	mainnetAddr := crypto.GenerateAddressWithPrefix(crypto.MainnetAddressPrefix, toSpend, toView)
	_, senderPub, _, err := crypto.DecodeAddress(testFromAddress)
	require.NoError(t, err)

	newArgs := func(to string) xcbuilder.TransferArgs {
		a, e := xcbuilder.NewTransferArgs(
			&xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero},
			xc.Address(testFromAddress), xc.Address(to),
			xc.NewAmountBlockchainFromUint64(5_000_000_000),
			xcbuilder.OptionPublicKey(senderPub), xcbuilder.OptionViewKey(testViewKey),
		)
		require.NoError(t, e)
		return a
	}

	// Mainnet address on a testnet-configured chain: rejected.
	testnetCfg := &xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero, Network: "testnet"}
	bTestnet, err := builder.NewTxBuilder(testnetCfg)
	require.NoError(t, err)
	_, err = bTestnet.Transfer(newArgs(mainnetAddr), makeTransferInput(t))
	require.ErrorContains(t, err, "configured for testnet")

	// Testnet address on a mainnet-configured chain: rejected.
	mainnetCfg := &xc.ChainBaseConfig{Chain: "XMR", Driver: xc.DriverMonero, Network: "mainnet"}
	bMainnet, err := builder.NewTxBuilder(mainnetCfg)
	require.NoError(t, err)
	_, err = bMainnet.Transfer(newArgs(testToAddress), makeTransferInput(t))
	require.ErrorContains(t, err, "configured for mainnet")

	// Matching network still builds.
	_, err = bTestnet.Transfer(newArgs(testToAddress), makeTransferInput(t))
	require.NoError(t, err)
}
