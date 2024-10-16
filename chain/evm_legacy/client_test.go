package evm_legacy_test

import (
	"context"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm_legacy"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client, err := evm_legacy.NewClient(&xc.ChainConfig{})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestFetchTxInput(t *testing.T) {

	vectors := []struct {
		name       string
		resp       interface{}
		val        *evm_legacy.TxInput
		err        string
		multiplier float64
	}{
		// Send ether normal tx
		{
			name: "fetchTxInput legacy",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_chainId
				`"0x1"`,
				// eth_gasPrice
				`"0xba43b7400"`,
				// eth_estimateGas
				`"0x52e4"`,
			},
			val: &evm_legacy.TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVMLegacy),
				Nonce:           6,
				GasLimit:        21220,
				GasPrice:        xc.NewAmountBlockchainFromUint64(50000000000),
				ChainId:         xc.NewAmountBlockchainFromUint64(1),
			},
			err:        "",
			multiplier: 1.0,
		},
		{
			name: "fetchTxInput legacy",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_chainId
				`"0x1"`,
				// eth_gasPrice
				`"0xba43b7400"`,
				// eth_estimateGas
				`"0x52e4"`,
			},
			val: &evm_legacy.TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVMLegacy),
				Nonce:           6,
				GasLimit:        21220,
				GasPrice:        xc.NewAmountBlockchainFromUint64(100000000000),
				ChainId:         xc.NewAmountBlockchainFromUint64(1),
			},
			err:        "",
			multiplier: 2.0,
		},
	}
	for _, v := range vectors {
		fmt.Println("testing ", v.name)
		server, close := testtypes.MockJSONRPC(t, v.resp)
		defer close()
		asset := &xc.ChainConfig{Chain: xc.ETH, Driver: xc.DriverEVMLegacy, URL: server.URL, ChainGasMultiplier: v.multiplier}
		client, err := evm_legacy.NewClient(asset)
		require.NoError(t, err)
		input, err := client.FetchLegacyTxInput(context.Background(), xc.Address(""), xc.Address(""))
		require.NoError(t, err)
		if v.err != "" {
			require.Equal(t, evm_legacy.TxInput{}, input)
			require.ErrorContains(t, err, v.err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, input)
			require.Equal(t, v.val, input)
		}
	}
}
