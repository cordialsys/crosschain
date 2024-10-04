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
		asset      xc.ITask
		txHash     string
		txResp     xrpClient.TransactionResponse
		ledgerResp xrpClient.LedgerResponse
		txInfo     xc.LegacyTxInfo
		err        string
	}{
		{
			asset:  &xc.ChainConfig{},
			txHash: "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
			txResp: xrpClient.TransactionResponse{
				xrpClient.TransactionResult{
					Account:            "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
					Amount:             "10000000",
					Destination:        "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
					Fee:                "12",
					Flags:              2147483648,
					LastLedgerSequence: 94538,
					Sequence:           92261,
					SigningPubKey:      "ED3F6EB32DDCFACD6128D245B7B8663391CEBFFF881310552B2C4911E267AAF81B",
					TransactionType:    "Payment",
					TxnSignature:       "1491B434F7DA81624D83F9C5F1CE82AAA3154715BCF1151E720531846E14D16A78F4BCBF4D35B012DC48189DA00888B5DF8F9221645C54FAB17B063BC122690F",
					Hash:               "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
					DeliverMax:         "10000000",
					TakerGets:          nil,
					TakerPays:          nil,
					CtID:               "C001711E00000001",
					Validated:          true,
					Date:               777656992,
					LedgerIndex:        94494,
					InLedger:           94494,
					Status:             "success",
				},
			},
			ledgerResp: xrpClient.LedgerResponse{
				Result: xrpClient.LedgerResult{
					Ledger: xrpClient.LedgerInfo{
						Closed:      false,
						LedgerIndex: "91190071",
						ParentHash:  "8BFD1FBA7E3E16C6F604DDB9DC235567D8D0C5F7BB62CF8A0A58B074937429C2",
					},
					LedgerCurrentIndex: 91190071,
					Validated:          false,
					Status:             "success",
				},
			},
			txInfo: xc.LegacyTxInfo{
				BlockHash:       "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
				TxID:            "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
				ExplorerURL:     "https://livenet.xrpl.org//tx/3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA?cluster=mainnet",
				From:            "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
				To:              "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromStr("10000000"),
				Fee:             xc.NewAmountBlockchainFromStr("12"),
				FeeContract:     "",
				BlockIndex:      94494,
				BlockTime:       777656992,
				Confirmations:   1144852,
				Status:          xc.TxStatus(0),
				Sources: []*xc.LegacyTxInfoEndpoint{
					{
						Address:                    "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
						ContractAddress:            "",
						Amount:                     xc.NewAmountBlockchainFromStr("10000000"),
						NativeAsset:                "",
						Asset:                      "",
						Memo:                       "",
						LegacyAptosContractAddress: "",
					},
				},
				Destinations: []*xc.LegacyTxInfoEndpoint{
					{
						Address:                    "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
						ContractAddress:            "",
						Amount:                     xc.NewAmountBlockchainFromStr("10000000"),
						NativeAsset:                "",
						Asset:                      "",
						Memo:                       "",
						LegacyAptosContractAddress: "",
					},
				},
				Time:         777656992,
				TimeReceived: 0,
				Error:        "",
			},
			err: "",
		},
	}

	for _, vector := range vectors {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)

			method := reqBody["method"].(string)
			if method == "tx" {
				// Respond with AccountInfoResponse
				json.NewEncoder(w).Encode(vector.txResp)
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

		client, _ := xrpClient.NewClient(&xc.ChainConfig{URL: server.URL})
		txInfo, err := client.FetchTxInfo(context.Background(), xc.TxHash(vector.txHash))

		if vector.err != "" {
			require.Equal(t, xc.LegacyTxInfo{}, txInfo)
			require.ErrorContains(t, err, vector.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, txInfo)
			//require.Equal(t, vector.txInfo.Amount, txInfo.Transfers[0].From[0].Amount)
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
