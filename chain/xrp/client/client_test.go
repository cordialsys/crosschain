package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xrpClient "github.com/cordialsys/crosschain/chain/xrp/client"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {

	client, err := xrpClient.NewClient(&xc.ChainConfig{})
	require.NotNil(t, client)
	require.Nil(t, err)
}

func TestFetchTxInput(t *testing.T) {

	vectors := []struct {
		asset           xc.ITask
		accountInfoResp interface{}
		ledgerResp      interface{}
		err             string
		expectedTxInput xrptxinput.TxInput
	}{
		{
			asset: &xc.ChainConfig{},
			accountInfoResp: xrpClient.AccountInfoResponse{
				Result: xrpClient.AccountInfoResultDetails{
					AccountData: xrpClient.AccountData{
						Sequence: 861823,
					},
				},
			},
			ledgerResp: xrpClient.LedgerResponse{
				Result: xrpClient.LedgerResult{
					LedgerCurrentIndex: 1221001,
				},
			},
			expectedTxInput: xrptxinput.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{
					Type: "xrp",
				},
				Sequence:           861823,
				LastLedgerSequence: 1221021,
				LegacyMemo:         "",
				PublicKey:          []uint8(nil),
			},
		},
		{
			asset: &xc.ChainConfig{},
			accountInfoResp: xrpClient.AccountInfoResponse{
				Result: xrpClient.AccountInfoResultDetails{
					AccountData: xrpClient.AccountData{},
				},
			},
			ledgerResp: xrpClient.LedgerResponse{
				Result: xrpClient.LedgerResult{
					LedgerCurrentIndex: 1221001,
				},
			},
			expectedTxInput: xrptxinput.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{
					Type: "xrp",
				},
				Sequence:           0,
				LastLedgerSequence: 1221021,
				LegacyMemo:         "",
				PublicKey:          []uint8(nil),
			},
		},
		{
			asset: &xc.ChainConfig{},
			accountInfoResp: xrpClient.AccountInfoResponse{
				Result: xrpClient.AccountInfoResultDetails{
					AccountData: xrpClient.AccountData{
						Sequence: 861823,
					},
				},
			},
			ledgerResp: xrpClient.LedgerResponse{
				Result: xrpClient.LedgerResult{},
			},
			expectedTxInput: xrptxinput.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{
					Type: "xrp",
				},
				Sequence:           861823,
				LastLedgerSequence: 20,
				LegacyMemo:         "",
				PublicKey:          []uint8(nil),
			},
		},
	}

	for _, vector := range vectors {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)

			method := reqBody["method"].(string)
			if method == "account_info" {
				// Respond with AccountInfoResponse
				json.NewEncoder(w).Encode(vector.accountInfoResp)
			} else if method == "ledger" {
				// Respond with LedgerResponse
				json.NewEncoder(w).Encode(vector.ledgerResp)
			} else {
				t.Errorf("unexpected method: %s", method)
			}
		}))
		defer server.Close()

		if token, ok := vector.asset.(*xc.TokenAssetConfig); ok {
			token.ChainConfig = &xc.ChainConfig{
				URL:   server.URL,
				Chain: "XRP",
			}
		} else {
			vector.asset.(*xc.ChainConfig).URL = server.URL
		}

		client, _ := xrpClient.NewClient(vector.asset)
		from := xc.Address("r92tsEZEjK82wra6xaDvjZocKnR78VqpEM")
		to := xc.Address("rs2x5gvFupB22myz86BUu7m5F4YuizsFna")
		input, err := client.FetchLegacyTxInput(context.Background(), from, to)

		if err != nil {
			require.Nil(t, input)
			require.ErrorContains(t, err, vector.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, input)
			txInput := input.(xc.TxInput)
			require.Equal(t, &vector.expectedTxInput, txInput)
		}
	}
}

//func TestSubmitTx(t *testing.T) {
//
//	client, _ := xrpClient.NewClient(&xc.ChainConfig{})
//	err := client.SubmitTx(context.Background(), &tx.Tx{})
//	require.EqualError(t, err, "not implemented")
//}

func TestFetchTxInfo(t *testing.T) {

	vectors := []struct {
		tx         string
		txResp     interface{}
		ledgerResp interface{}
		val        xc.LegacyTxInfo
		err        string
	}{
		{
			tx: "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
			txResp: []string{`{"name": "chains/XRP/transactions/3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA", "hash": "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
					   "chain": "XRP",
"block": {
  "height": 94494,
  "hash": "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
  "time": "1994-08-23T18:49:52+03:00"
},
"transfers": [
  {
    "from": [
      {
        "asset": "chains/XRP/assets/XRP",
        "contract": "XRP",
        "balance": "10000000",
        "address": "chains/XRP/addresses/rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S"
      }
    ],
    "to": [
      {
        "asset": "chains/XRP/assets/XRP",
        "contract": "XRP",
        "balance": "10000000",
        "address": "chains/XRP/addresses/rLETt614usCXtkc8YcQmrzachrCaDjACjP"
      }
    ]
  },
  {
    "from": [
      {
        "asset": "chains/XRP/assets/XRP",
        "contract": "XRP",
        "balance": "12",
        "address": "chains/XRP/addresses/rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S"
      }
    ],
    "to": []
  }
],
"fees": [
  {
    "asset": "chains/XRP/assets/XRP",
    "contract": "XRP",
    "balance": "12"
  }
],
"confirmations": 587956
}`},
			ledgerResp: []string{`{}`},
			val:        xc.LegacyTxInfo{},
			err:        "",
		},
	}

	for i, v := range vectors {
		fmt.Println("test case", i)
		server, close := testtypes.MockJSONRPC(t, v.txResp)
		defer close()

		client, _ := xrpClient.NewClient(&xc.ChainConfig{URL: server.URL})
		txInfo, err := client.FetchTxInfo(context.Background(), xc.TxHash(v.tx))

		if v.err != "" {
			require.Equal(t, xc.LegacyTxInfo{}, txInfo)
			require.ErrorContains(t, err, v.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, txInfo)
			require.Equal(t, v.val, txInfo)
		}
	}
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

		balance, err := client.FetchBalance(context.Background(), address)

		if v.err != "" {
			require.Equal(t, "0", balance.String())
			require.ErrorContains(t, err, v.err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, balance)
			assert.Equal(t, balance, v.val)
		}
	}
}

func TestFetchCBalance(t *testing.T) {
	vectors := []struct {
		resp interface{}
		val  string
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
			"20000000",
			"",
		},
		{
			xrpClient.AccountLinesResponse{},
			"0",
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

		balance, err := client.FetchBalance(context.Background(), address)

		if v.err != "" {
			require.Equal(t, "0", balance.String())
			require.ErrorContains(t, err, v.err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, balance)
			humanReadbleBalance, _ := xc.NewAmountHumanReadableFromStr(v.val)
			assert.Equal(t, balance, humanReadbleBalance.ToBlockchain(15))
		}
	}
}
