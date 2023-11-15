package utils_test

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/utils"
	"github.com/test-go/testify/require"
)

type TxTestInput struct {
	utils.TxPriceInput
	A string
}

func TestTxPriceInput(t *testing.T) {
	inp := &TxTestInput{
		A: "A",
	}
	amount, err := xc.NewAmountHumanReadableFromStr("1.2")
	require.NoError(t, err)
	inp.SetUsdPrice(xc.HASH, "1234", amount)

	amountGet, ok := inp.GetUsdPrice(xc.HASH, "1234")
	require.True(t, ok)
	require.Equal(t, "1.2", amountGet.String())

	_, ok = inp.GetUsdPrice(xc.HASH, "12345")
	require.False(t, ok)
	_, ok = inp.GetUsdPrice(xc.ETH, "1234")
	require.False(t, ok)

	bz, err := json.Marshal(inp)
	require.NoError(t, err)

	inp2 := &TxTestInput{}
	err = json.Unmarshal(bz, inp2)
	require.NoError(t, err)

	amountGet, ok = inp.GetUsdPrice(xc.HASH, "1234")
	require.True(t, ok)
	require.Equal(t, "1.2", amountGet.String())
	require.Equal(t, "A", inp2.A)

	vector := `{"prices":[{"contract":"12345","chain":"HASH","price_usd":"2.3"}],"A":"A"}`
	inp3 := &TxTestInput{}
	err = json.Unmarshal([]byte(vector), inp3)
	require.NoError(t, err)
	amountGet, ok = inp3.GetUsdPrice(xc.HASH, "12345")
	require.True(t, ok)
	require.Equal(t, "2.3", amountGet.String())
}
