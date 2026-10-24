package sui

import (
	"encoding/json"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/stretchr/testify/require"
)

func TestTransactionIDOption(t *testing.T) {
	input := NewTxInput()
	_, err := builder.WithTxInputOptions(input, 0, "", "")
	require.NoError(t, err)
	require.Empty(t, input.TransactionID)
	_, err = builder.WithTxInputOptions(input, 0, "", "transfer-id")
	require.NoError(t, err)
	require.Equal(t, "transfer-id", input.TransactionID)
}

func protectedInput(id string) *TxInput {
	in := NewTxInput()
	in.CurrentEpoch = 42
	in.SetTransactionID(id)
	in.PaymentKit = &PaymentKitInput{PackageID: "pkg", RegistryID: "registry", Sender: xc.Address("0x" + strings.Repeat("11", 32)), ChainID: "chain", CoinType: NativeCoin, RetentionEpochs: 30}
	in.AddressBalance = &AddressBalanceInput{}
	return in
}
func TestPaymentKitNonce(t *testing.T) {
	from := xc.Address("0x" + strings.Repeat("11", 32))
	other := xc.Address("0x" + strings.Repeat("22", 32))
	a, err := PaymentKitNonce(from, "42")
	require.NoError(t, err)
	require.Len(t, a, 32)
	b, err := PaymentKitNonce(from, "42")
	require.NoError(t, err)
	require.Equal(t, a, b)
	for _, tc := range []struct {
		from xc.Address
		id   string
	}{{other, "42"}, {from, "43"}, {from, "你好/" + strings.Repeat("long", 100)}} {
		b, err = PaymentKitNonce(tc.from, tc.id)
		require.NoError(t, err)
		require.NotEqual(t, a, b)
		require.Len(t, b, 32)
	}
	_, err = PaymentKitNonce(from, "")
	require.Error(t, err)
	_, err = PaymentKitNonce(xc.Address("0x"+strings.Repeat("11", 33)), "42")
	require.Error(t, err)
	for n, want := range map[string]string{
		"0xa5016862fdccba7cc576b56cc5a391eda6775200aaa03a6b3c97d512312878db": "0x4960b8dc38cc80df9a548bd72ce09d157e4f5e32c570973cb54c9a634bcaa0d8",
		"0xccd3e4c7802921991cd9ce488c4ca0b51334ba75483702744242284ccf3ae7c2": "0x0481b22fd3c73f3176db4c20419e1d09c6a3074aeed65acdc776f7359285ffd2",
	} {
		got, err := PaymentKitRegistryID(n)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}
func TestPaymentKitConflictRules(t *testing.T) {
	old := protectedInput("42")
	newer := protectedInput("42")
	require.False(t, newer.IndependentOf(old))
	require.True(t, newer.SafeFromDoubleSend(old))
	newer.SetTransactionID("43")
	require.True(t, newer.IndependentOf(old))
	require.False(t, newer.SafeFromDoubleSend(old))
	newer.SetTransactionID("42")
	newer.CurrentEpoch = 71
	require.True(t, newer.SafeFromDoubleSend(old))
	newer.CurrentEpoch = 72
	require.False(t, newer.SafeFromDoubleSend(old))
	require.False(t, newer.IndependentOf(old))
	newer.CurrentEpoch = 41
	require.False(t, newer.SafeFromDoubleSend(old))
	newer = protectedInput("42")
	newer.PaymentKit.RegistryID = "other"
	require.False(t, newer.SafeFromDoubleSend(old))
	newer = protectedInput("42")
	newer.PaymentKit.Sender = "other"
	require.False(t, newer.SafeFromDoubleSend(old))
	newer = protectedInput("42")
	newer.PaymentKit.ChainID = "other"
	require.False(t, newer.SafeFromDoubleSend(old))
	require.False(t, newer.IndependentOf(old))
	legacy := NewTxInput()
	legacy.CurrentEpoch = 42
	require.False(t, newer.SafeFromDoubleSend(legacy))
	wire, err := json.Marshal(old)
	require.NoError(t, err)
	var decoded TxInput
	require.NoError(t, json.Unmarshal(wire, &decoded))
	require.True(t, decoded.SafeFromDoubleSend(old))
	// Object dependencies still matter, including additional sponsor gas coins.
	old = protectedInput("42")
	newer = protectedInput("43")
	coin := &types.Coin{}
	require.NoError(t, json.Unmarshal([]byte(`{"coinObjectId":"0x1111111111111111111111111111111111111111111111111111111111111111","version":"1","digest":"11111111111111111111111111111111","balance":"100"}`), coin))
	old.AddressBalance.GasCoins = []*types.Coin{coin}
	newer.Coins = []*types.Coin{coin}
	require.False(t, newer.IndependentOf(old))
	newer.CurrentEpoch++
	require.True(t, newer.IndependentOf(old))
}
