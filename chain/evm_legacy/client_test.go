package evm_legacy

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
)

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(&xc.ChainConfig{})
	require.NoError(err)
	require.NotNil(client)
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()

	vectors := []struct {
		name       string
		resp       interface{}
		val        *TxInput
		err        string
		multiplier float64
	}{
		// Send ether normal tx
		{
			name: "fetchTxInput legacy",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_gasPrice
				`"0xba43b7400"`,
				// eth_estimateGas
				`"0x52e4"`,
			},
			val: &TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVMLegacy),
				Nonce:           6,
				GasLimit:        21220,
				GasPrice:        xc.NewAmountBlockchainFromUint64(50000000000),
			},
			err:        "",
			multiplier: 1.0,
		},
		{
			name: "fetchTxInput legacy",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_gasPrice
				`"0xba43b7400"`,
				// eth_estimateGas
				`"0x52e4"`,
			},
			val: &TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVMLegacy),
				Nonce:           6,
				GasLimit:        21220,
				GasPrice:        xc.NewAmountBlockchainFromUint64(100000000000),
			},
			err:        "",
			multiplier: 2.0,
		},
	}
	for _, v := range vectors {
		fmt.Println("testing ", v.name)
		server, close := testtypes.MockJSONRPC(s.T(), v.resp)
		defer close()
		asset := &xc.ChainConfig{Chain: xc.ETH, Driver: xc.DriverEVMLegacy, URL: server.URL, ChainGasMultiplier: v.multiplier}
		client, err := NewClient(asset)
		require.NoError(err)
		input, err := client.FetchLegacyTxInput(s.Ctx, xc.Address(""), xc.Address(""))
		require.NoError(err)
		if v.err != "" {
			require.Equal(TxInput{}, input)
			require.ErrorContains(err, v.err)
		} else {
			require.Nil(err)
			require.NotNil(input)
			require.Equal(v.val, input)
		}
	}
}
