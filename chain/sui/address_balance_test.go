package sui

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	"github.com/stretchr/testify/require"
)

func TestAddressBalanceInputPrecisionAndRetrySafety(t *testing.T) {
	input := NewTxInput()
	input.CurrentEpoch = 42
	input.AddressBalance = &AddressBalanceInput{Balance: math.MaxUint64, GasBalance: 9_007_199_254_740_993}
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"balance":"18446744073709551615"`)
	require.Contains(t, string(encoded), `"gas_balance":"9007199254740993"`)
	var decoded TxInput
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, input.AddressBalance, decoded.AddressBalance)
	require.False(t, input.SafeFromDoubleSend(&decoded))
	legacy := NewTxInput()
	legacy.CurrentEpoch = 42
	require.False(t, legacy.SafeFromDoubleSend(input))
	require.False(t, input.SafeFromDoubleSend(legacy))
	input.CurrentEpoch++
	require.False(t, input.SafeFromDoubleSend(&decoded))
	require.True(t, input.IndependentOf(&decoded))
	require.False(t, decoded.SafeFromDoubleSend(input))
	require.Equal(t, "18446744073709551615", input.TotalBalance().String())
}

func TestCoinTypeParsing(t *testing.T) {
	for _, value := range []string{NativeCoin, "0x" + strings.Repeat("0", 63) + "2::sui::SUI", "0x42::lp::LP<0x2::sui::SUI, vector<u64>>"} {
		tag, err := parseCoinType(value)
		require.NoError(t, err)
		wire, err := tag.BcsSerialize()
		require.NoError(t, err)
		decoded, err := bcs.BcsDeserializeTypeTag(wire)
		require.NoError(t, err)
		roundtrip, err := decoded.BcsSerialize()
		require.NoError(t, err)
		require.Equal(t, wire, roundtrip)
	}
	for _, value := range []string{"", "u64", "0x2::sui::", "0x2::bad-module::SUI", "0xzz::sui::SUI", "0x2::c::C<>", "0x2::c::C<u64", "0x2::sui::SUI garbage", "0x2::c::C<" + strings.Repeat("vector<", 34) + "u8" + strings.Repeat(">", 35)} {
		_, err := parseCoinType(value)
		require.Error(t, err, value)
	}
	input := NewTxInput()
	input.AddressBalance = &AddressBalanceInput{CoinType: "0x" + strings.Repeat("0", 63) + "2::sui::SUI"}
	require.True(t, input.IsNativeTransfer())
}

func TestInvalidAddressFunding(t *testing.T) {
	from := xc.Address("0x" + strings.Repeat("11", 32))
	input := conflictInput(t, "42")
	input.GasCoinOwner = from
	input.GasBudget = 1
	input.AddressBalance = &AddressBalanceInput{Sender: from, CoinType: NativeCoin, Balance: 100, GasBalance: 100, ChainID: "invalid"}
	builder, _ := NewTxBuilder(conflictBuilderConfig())
	_, err := builder.NewTransfer(from, make([]byte, 32), from, from, xc.NewAmountBlockchainFromUint64(10), input)
	require.ErrorContains(t, err, "chain ID")
	_, err = input.balanceFunding(from, from, xc.NewAmountBlockchainFromStr("18446744073709551616"))
	require.ErrorContains(t, err, "uint64")
	_, err = input.balanceFunding(from, from, xc.NewAmountBlockchainFromStr("-1"))
	require.ErrorContains(t, err, "uint64")
	_, err = input.balanceFunding("other", from, xc.NewAmountBlockchainFromUint64(1))
	require.ErrorContains(t, err, "owner")
	input.AddressBalance.GasBalance = 1
	_, err = input.balanceFunding(from, from, xc.NewAmountBlockchainFromUint64(1))
	require.ErrorContains(t, err, "inconsistent")
}
