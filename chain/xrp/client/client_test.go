package client_test

import (
	"context"
	"fmt"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xrpClient "github.com/cordialsys/crosschain/chain/xrp/client"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {

	client, err := xrpClient.NewClient(&xc.ChainConfig{})
	require.NotNil(t, client)
	require.NotNil(t, err)
}

func TestFetchTxInput(t *testing.T) {

	client, _ := xrpClient.NewClient(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchLegacyTxInput(context.Background(), from, to)
	require.NotNil(t, input)
	require.EqualError(t, err, "not implemented")
}

func TestSubmitTx(t *testing.T) {

	client, _ := xrpClient.NewClient(&xc.ChainConfig{})
	err := client.SubmitTx(context.Background(), &tx.Tx{})
	require.EqualError(t, err, "not implemented")
}

func TestFetchTxInfo(t *testing.T) {

	client, _ := xrpClient.NewClient(&xc.ChainConfig{})
	info, err := client.FetchLegacyTxInfo(context.Background(), xc.TxHash("hash"))
	require.NotNil(t, info)
	require.EqualError(t, err, "not implemented")
}

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestFetchNativeBalance(t *testing.T) {

	vectors := []struct {
		resp interface{}
		val  xc.AmountBlockchain
		err  string
	}{
		{
			xrpClient.AccountInfoResponse{
				Result: xrpClient.AccountInfoResultDetails{
					AccountData: xrpClient.AccountData{
						Balance: "20000000",
					},
				},
			},
			xc.NewAmountBlockchainFromUint64(20000000),
			"",
		},
		{
			xrpClient.AccountInfoResponse{},
			xc.NewAmountBlockchainFromUint64(0),
			"empty balance returned for account: r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
		},
	}

	for i, v := range vectors {
		fmt.Println("testcase ", i)
		server, close := testtypes.MockJSONRPC(t, v.resp)
		defer close()

		client, _ := xrpClient.NewClient(&xc.ChainConfig{URL: server.URL})

		address := xc.Address("r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59")

		result, err := client.FetchBalance(context.Background(), address)

		if v.err != "" {
			require.Equal(t, "0", result.String())
			require.ErrorContains(t, err, v.err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, result)
			assert.Equal(t, result, v.val)
		}
	}
}

func TestFetchBalance(t *testing.T) {
	vectors := []struct {
		resp interface{}
		val  xc.AmountBlockchain
		err  string
	}{
		{
			xrpClient.AccountLinesResponse{
				Result: xrpClient.AccountLinesResultDetails{
					Lines: []xrpClient.Line{
						{
							Account:  "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB",
							Currency: "FMT",
							Balance:  "20000000",
						},
					},
				},
			},
			xc.NewAmountBlockchainFromUint64(20000000),
			"",
		},
		{
			xrpClient.AccountLinesResponse{},
			xc.NewAmountBlockchainFromUint64(0),
			"empty balance returned for account: rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB",
		},
	}

	for i, v := range vectors {
		fmt.Println("testcase ", i)
		server, close := testtypes.MockJSONRPC(t, v.resp)
		defer close()

		chain := xc.ChainConfig{URL: server.URL}

		client, _ := xrpClient.NewClient(&xc.TokenAssetConfig{
			Contract:    "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB",
			Chain:       chain.Chain,
			ChainConfig: &chain,
			Decimals:    0,
		})

		address := xc.Address("rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")

		result, err := client.FetchBalance(context.Background(), address)

		if v.err != "" {
			require.Equal(t, "0", result.String())
			require.ErrorContains(t, err, v.err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, result)
			assert.Equal(t, result, v.val)
		}
	}
}
