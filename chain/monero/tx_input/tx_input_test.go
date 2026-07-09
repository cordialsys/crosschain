package tx_input_test

import (
	"math"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"github.com/cordialsys/crosschain/pkg/hex"
	"github.com/stretchr/testify/require"
)

func makeInput(outputs ...tx_input.Output) *tx_input.TxInput {
	input := tx_input.NewTxInput()
	input.Outputs = outputs
	return input
}

func TestSelectOutputs(t *testing.T) {
	// PerByteFee 0 -> flat minimum fee of 100_000_000 regardless of input count.
	input := tx_input.NewTxInput()
	outputs := []tx_input.Output{
		{Amount: 1_000_000_000, GlobalIndex: 3, TxHash: "c"},
		{Amount: 10_000_000_000, GlobalIndex: 2, TxHash: "b"},
		{Amount: 10_000_000_000, GlobalIndex: 1, TxHash: "a"},
	}

	// Largest output covers amount+fee; the rest are tacked on as dust
	// consolidation (all fit under the 10-output cap). Ties broken by global
	// index; dust appended smallest-first.
	selected, total := input.SelectOutputs(outputs, 5_000_000_000)
	require.Len(t, selected, 3)
	require.EqualValues(t, 1, selected[0].GlobalIndex)
	require.EqualValues(t, 3, selected[1].GlobalIndex)
	require.EqualValues(t, 2, selected[2].GlobalIndex)
	require.EqualValues(t, 21_000_000_000, total)

	// Insufficient: returns everything, builder reports the error.
	selected, total = input.SelectOutputs(outputs, 50_000_000_000)
	require.Len(t, selected, 3)
	require.EqualValues(t, 21_000_000_000, total)

	// Selection is deterministic regardless of input order.
	selected, _ = input.SelectOutputs(outputs, 15_000_000_000)
	reordered := []tx_input.Output{outputs[2], outputs[0], outputs[1]}
	selected2, _ := input.SelectOutputs(reordered, 15_000_000_000)
	require.Equal(t, selected, selected2)
}

func TestSelectOutputsCapsAtMax(t *testing.T) {
	input := tx_input.NewTxInput()
	var outputs []tx_input.Output
	for i := 0; i < 15; i++ {
		outputs = append(outputs, tx_input.Output{
			Amount:      10_000_000_000,
			GlobalIndex: uint64(i),
		})
	}
	selected, _ := input.SelectOutputs(outputs, 5_000_000_000)
	require.Len(t, selected, tx_input.MaxSpendableOutputs)
	// Minimum set (largest first w/ tie on index -> index 0), then dust from
	// the smallest end of the sort order.
	require.EqualValues(t, 0, selected[0].GlobalIndex)
	require.EqualValues(t, 14, selected[1].GlobalIndex)
}

func TestSelectOutputsUncappedWhenAmountRequiresIt(t *testing.T) {
	// MaxSpendableOutputs only caps dust padding; the minimum covering set
	// takes as many outputs as the amount requires.
	input := tx_input.NewTxInput()
	var outputs []tx_input.Output
	for i := 0; i < 20; i++ {
		outputs = append(outputs, tx_input.Output{
			Amount:      1_000_000_000,
			GlobalIndex: uint64(i),
		})
	}
	// 15 outputs needed to cover amount + min fee (14.9e9 + 1e8).
	selected, total := input.SelectOutputs(outputs, 14_900_000_000)
	require.Len(t, selected, 15)
	require.EqualValues(t, 15_000_000_000, total)
}

func TestSelectOutputsDustMustCoverItsFee(t *testing.T) {
	// With a real per-byte fee, each extra input costs ~800 bytes * fee. A
	// dust output worth less than its marginal fee must not be consolidated
	// when it would break coverage.
	input := tx_input.NewTxInput()
	// High enough that the size-based fee exceeds the flat minimum:
	// feeFor(1) = 230_000_000, feeFor(2) = 310_000_000 (marginal 80_000_000).
	input.PerByteFee = 100_000
	outputs := []tx_input.Output{
		{Amount: 1_230_000_000, GlobalIndex: 1}, // covers 1e9 + feeFor(1), nothing to spare
		{Amount: 1_000_000, GlobalIndex: 2},     // dust worth less than the marginal fee
	}
	selected, total := input.SelectOutputs(outputs, 1_000_000_000)
	require.Len(t, selected, 1)
	require.EqualValues(t, 1, selected[0].GlobalIndex)
	require.EqualValues(t, 1_230_000_000, total)

	// With headroom, the dust gets consolidated.
	outputs[0].Amount = 2_000_000_000
	selected, _ = input.SelectOutputs(outputs, 1_000_000_000)
	require.Len(t, selected, 2)
}

func TestIndependentOf(t *testing.T) {
	a := makeInput(tx_input.Output{PublicKey: hex.Hex("p1")})
	b := makeInput(tx_input.Output{PublicKey: hex.Hex("p2")})
	shared := makeInput(
		tx_input.Output{PublicKey: hex.Hex("p1")},
		tx_input.Output{PublicKey: hex.Hex("p3")},
	)

	require.True(t, a.IndependentOf(b))
	require.True(t, b.IndependentOf(a))
	require.False(t, a.IndependentOf(shared))
	require.False(t, shared.IndependentOf(a))

	// Non-utxo input types are not independent by default.
	require.False(t, a.IndependentOf(xc.TxInput(nil)))
}

// TestIndependentOfIdentityIsKeyImage pins the fix for the spoofable-identity
// bug: two coins are "the same" iff they share the one-time public key (which
// determines the key image), regardless of the tx_hash/index an untrusted
// indexer stamps on them.
func TestIndependentOfIdentityIsKeyImage(t *testing.T) {
	// Same (tx_hash, index) label but different coins (different one-time keys):
	// these must be treated as INDEPENDENT, or a double-send would slip through.
	a := makeInput(tx_input.Output{TxHash: "tx1", Index: 0, PublicKey: hex.Hex("pA")})
	forged := makeInput(tx_input.Output{TxHash: "tx1", Index: 0, PublicKey: hex.Hex("pB")})
	require.True(t, a.IndependentOf(forged))
	require.False(t, a.SafeFromDoubleSend(forged),
		"different key images must not be reported safe just because labels collide")

	// Different (tx_hash, index) labels but the same one-time key: the same coin,
	// so NOT independent and safe from double-send.
	relabeled := makeInput(tx_input.Output{TxHash: "tx9", Index: 7, PublicKey: hex.Hex("pA")})
	require.False(t, a.IndependentOf(relabeled))
	require.True(t, a.SafeFromDoubleSend(relabeled))
}

// TestGetFeeLimitAccurate: GetFeeLimit reflects the fee for the actual number
// of inputs the tx will spend (what the builder pays), not a fixed guess.
func TestGetFeeLimitAccurate(t *testing.T) {
	input := tx_input.NewTxInput()
	input.PerByteFee = 100_000
	for i := 0; i < 4; i++ {
		input.Outputs = append(input.Outputs, tx_input.Output{PublicKey: hex.Hex{byte('a' + i)}})
	}
	fee, asset := input.GetFeeLimit()
	require.Equal(t, xc.ContractAddress(""), asset)
	// Must equal the fee the builder computes for these 4 inputs.
	require.Equal(t, input.EstimatedFeeFor(len(input.Outputs)), fee.Uint64())
	// And differ from the fee for a different input count (proves it's not fixed).
	require.NotEqual(t, input.EstimatedFeeFor(10), fee.Uint64())
}

// TestFeeNoOverflow: a hostile fee estimate must not wrap a huge fee down to a
// small one. EstimatedFeeFor clamps up; GetFeeLimit reports the exact large
// value so the framework's fee-limit check can reject it.
func TestFeeNoOverflow(t *testing.T) {
	input := tx_input.NewTxInput()
	input.PerByteFee = math.MaxUint64
	input.Outputs = []tx_input.Output{{PublicKey: hex.Hex("a")}}

	require.EqualValues(t, uint64(math.MaxUint64), input.EstimatedFeeFor(1),
		"overflowing fee must clamp up, never wrap to a small value")

	fee, _ := input.GetFeeLimit()
	// Exact value exceeds uint64, so a configured fee_limit (a small amount)
	// will always be smaller -> CheckFeeLimit rejects.
	require.False(t, fee.Int().IsUint64(), "exact fee should exceed uint64")
	maxU64 := xc.NewAmountBlockchainFromUint64(math.MaxUint64)
	require.Positive(t, fee.Cmp(&maxU64))
}

// TestSetGasFeePriorityNoWrap: scaling an absurd per-byte fee must not wrap
// through int64 into a negative/garbage value.
func TestSetGasFeePriorityNoWrap(t *testing.T) {
	input := tx_input.NewTxInput()
	input.PerByteFee = 20_000
	require.NoError(t, input.SetGasFeePriority(xc.Aggressive)) // 1.5x
	require.EqualValues(t, 30_000, input.PerByteFee)

	// A value above MaxInt64 previously cast to negative; now it stays sane.
	input.PerByteFee = math.MaxUint64
	err := input.SetGasFeePriority(xc.Aggressive)
	require.Error(t, err, "scaling MaxUint64 by 1.5x overflows uint64 and should error, not wrap")
}

func TestSafeFromDoubleSend(t *testing.T) {
	a := makeInput(tx_input.Output{PublicKey: hex.Hex("p1")})
	sameOutput := makeInput(tx_input.Output{PublicKey: hex.Hex("p1")})
	disjoint := makeInput(tx_input.Output{PublicKey: hex.Hex("p2")})
	overlapping := makeInput(
		tx_input.Output{PublicKey: hex.Hex("p1")},
		tx_input.Output{PublicKey: hex.Hex("p3")},
	)

	// Same or overlapping outputs -> same key image -> at most one confirms.
	require.True(t, a.SafeFromDoubleSend(sameOutput))
	require.True(t, a.SafeFromDoubleSend(overlapping))
	// Disjoint outputs could both confirm.
	require.False(t, a.SafeFromDoubleSend(disjoint))
	// Unknown input types are never safe.
	require.False(t, a.SafeFromDoubleSend(xc.TxInput(nil)))
}
