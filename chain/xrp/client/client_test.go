package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	xc "github.com/cordialsys/crosschain"
	xrpClient "github.com/cordialsys/crosschain/chain/xrp/client"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	xrptx "github.com/cordialsys/crosschain/chain/xrp/tx"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
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
			accountInfoResp: types.AccountInfoResponse{
				Result: types.AccountInfoResultDetails{
					AccountData: types.AccountData{
						Sequence: 861823,
					},
				},
			},
			ledgerResp: types.LedgerResponse{
				Result: types.LedgerResult{
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
			accountInfoResp: types.AccountInfoResponse{
				Result: types.AccountInfoResultDetails{
					AccountData: types.AccountData{},
				},
			},
			ledgerResp: types.LedgerResponse{
				Result: types.LedgerResult{
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
			accountInfoResp: types.AccountInfoResponse{
				Result: types.AccountInfoResultDetails{
					AccountData: types.AccountData{
						Sequence: 861823,
					},
				},
			},
			ledgerResp: types.LedgerResponse{
				Result: types.LedgerResult{},
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

func TestSubmitTx(t *testing.T) {

	vectors := []struct {
		txInput    xc.Tx
		submitResp types.SubmitResponse
		asset      xc.ITask
	}{
		{
			txInput: &xrptx.Tx{
				XRPTx: &xrptx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: xrptx.AmountBlockchain{
						XRPAmount: "10000000",
					},
					Destination:        "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:     0,
					Fee:                "10",
					Flags:              0,
					LastLedgerSequence: 1314663,
					Sequence:           861824,
					SigningPubKey:      "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType:    "Payment",
					TxnSignature:       "30450221009e7787de5b11bc17eaec2bd5841879434ce19fbe6f137eff0cd919ba435b236b02205f6afbcea8cc0f0cdda593a72f67aaeeb37b955ef4b523ed2967c2bade3c322b",
				},
			},
			submitResp: types.SubmitResponse{
				Result: types.SubmitResult{
					Accepted:                 true,
					AccountSequenceAvailable: 861827,
					AccountSequenceNext:      861827,
					Applied:                  true,
					Broadcast:                true,
					EngineResult:             "tesSUCCESS",
					EngineResultCode:         0,
					EngineResultMessage:      "The transaction was applied. Only final in a validated ledger.",
					Kept:                     true,
					OpenLedgerCost:           "10",
					Queued:                   false,
					TxBlob:                   "120000220000000024000D26822E00000000201B0014104261400000000098968068400000000000000A73210391E85C96FEAB1C71250308EF99375BB3FA9B846FC2C8B906976FA9AC4BED085774463044022047239A5473D9830F8D7379D931FCB869A40F1CAA7082901258274815D8F7B5E30220294C7CA1B3ADB4702CF3A43892C8AB8BBA6AF160492B0A595C396146A6D1CA1B81145E29568B3CD06772650182A436111F283A91A51F83141C5C7D6FFB375B5A656CC0E80E20F1C8CA2E68BB",
					ValidatedLedgerIndex:     1314863,
					Status:                   "success",
				},
			},
		},
	}

	for testNo, vector := range vectors {
		fmt.Println("testcase ", testNo)
		server, close := testtypes.MockJSONRPC(t, vector.submitResp)
		defer close()

		client, _ := xrpClient.NewClient(&xc.ChainConfig{URL: server.URL})

		err := client.SubmitTx(context.Background(), vector.txInput)
		require.NoError(t, err)
	}
}

func TestFetchTxInfo(t *testing.T) {

	vectors := []struct {
		asset          xc.ITask
		txHash         string
		txResp         string
		ledgerResp     types.LedgerResponse
		expectedTxInfo xclient.TxInfo
		err            string
	}{
		{
			asset:  &xc.ChainConfig{},
			txHash: "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
			txResp: `{
			  "result": {
				"Account": "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
				"Amount": "10000000",
				"Destination": "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
				"Fee": "12",
				"Flags": 2147483648,
				"LastLedgerSequence": 94538,
				"Sequence": 92261,
				"SigningPubKey": "ED3F6EB32DDCFACD6128D245B7B8663391CEBFFF881310552B2C4911E267AAF81B",
				"TransactionType": "Payment",
				"TxnSignature": "1491B434F7DA81624D83F9C5F1CE82AAA3154715BCF1151E720531846E14D16A78F4BCBF4D35B012DC48189DA00888B5DF8F9221645C54FAB17B063BC122690F",
				"hash": "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
				"DeliverMax": "10000000",
				"ctid": "C001711E00000001",
				"meta": {
				  "AffectedNodes": [
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "Account": "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
						  "Balance": "20000000",
						  "Flags": 0,
						  "OwnerCount": 0,
						  "Sequence": 92557
						},
						"LedgerEntryType": "AccountRoot",
						"LedgerIndex": "18F34B88295C6BBD378F0F94E600660C8B7CDEAC89A3C41236910B3334F352FE",
						"PreviousFields": {
						  "Balance": "10000000"
						},
						"PreviousTxnID": "364A0DB3EDF04CB1661C29F6120224FB8984FBED51F4DEC431E5D8BEE61BF00F",
						"PreviousTxnLgrSeq": 92557
					  }
					},
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "Account": "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
						  "Balance": "79999976",
						  "Flags": 0,
						  "OwnerCount": 0,
						  "Sequence": 92262
						},
						"LedgerEntryType": "AccountRoot",
						"LedgerIndex": "373EBB701A602BFB0D5D1648D7361A3E5D40FD2FD3FA6D5FA9B5CD73E4AE7003",
						"PreviousFields": {
						  "Balance": "89999988",
						  "Sequence": 92261
						},
						"PreviousTxnID": "364A0DB3EDF04CB1661C29F6120224FB8984FBED51F4DEC431E5D8BEE61BF00F",
						"PreviousTxnLgrSeq": 92557
					  }
					}
				  ],
				  "TransactionIndex": 0,
				  "TransactionResult": "tesSUCCESS",
				  "delivered_amount": "10000000"
				},
				"validated": true,
				"date": 777656992,
				"ledger_index": 94494,
				"inLedger": 94494,
				"status": "success"
			  }
			}
			`,
			ledgerResp: types.LedgerResponse{
				Result: types.LedgerResult{
					Ledger: types.LedgerInfo{
						Closed:      false,
						LedgerIndex: "91190071",
						ParentHash:  "8BFD1FBA7E3E16C6F604DDB9DC235567D8D0C5F7BB62CF8A0A58B074937429C2",
					},
					LedgerCurrentIndex: 91190071,
					Validated:          false,
					Status:             "success",
				},
			},
			expectedTxInfo: xclient.TxInfo{
				Name:  "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
				Hash:  "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
				Chain: "",
				Block: &xclient.Block{
					Height: 94494,
					Hash:   "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA",
				},
				Transfers: []*xclient.Transfer{
					{
						From: []*xclient.BalanceChange{
							{
								Asset:    "chains/assets",
								Contract: "",
								Balance:  xc.NewAmountBlockchainFromStr("10000012000000"),
								Amount:   nil,
								Address:  "chains/addresses/rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
							},
						},
						To: []*xclient.BalanceChange{
							{
								Asset:    "chains/assets",
								Contract: "",
								Balance:  xc.NewAmountBlockchainFromStr("10000000000000"),
								Amount:   nil,
								Address:  "chains/addresses/rLETt614usCXtkc8YcQmrzachrCaDjACjP",
							},
						},
						Memo: "",
					},
				},
				Fees: []*xclient.Balance{
					{
						Asset:    "chains/assets",
						Contract: "",
						Balance:  xc.NewAmountBlockchainFromStr("12000000"),
						Amount:   nil,
					},
				},
				Confirmations: 91097810,
			},
			err: "",
		},
		{
			asset:  &xc.ChainConfig{},
			txHash: "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
			txResp: `
			{
			  "result": {
				"Account": "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
				"Fee": "12",
				"Flags": 786432,
				"LastLedgerSequence": 90659227,
				"Sequence": 90659082,
				"SigningPubKey": "03096E30DF354C174D22ACD99C201FCE1CC6EE588D58F11CF858A45FDE4FCF0C6E",
				"TakerGets": "4862466",
				"TakerPays": {
				  "currency": "USD",
				  "issuer": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
				  "value": "2.5"
				},
				"TransactionType": "OfferCreate",
				"TxnSignature": "3045022100C77D56EF2F3B4995D9F021D78613490915A5BE8AC3F7BFEE8BEEED3C81B646E40220650629F4D40B3803D14A402926482F042841AA33DB617C6B8CA63DFC85E87188",
				"hash": "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
				"ctid": "C567599300010000",
				"meta": {
				  "AffectedNodes": [
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "Account": "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
						  "Balance": "32008483",
						  "Flags": 0,
						  "OwnerCount": 1,
						  "Sequence": 90659083
						},
						"LedgerEntryType": "AccountRoot",
						"LedgerIndex": "2CD4DCB5BAE3A17AA69B12101056D4AB5A91269D5A1132DEF611019B9A3E1DC5",
						"PreviousFields": {
						  "Balance": "36870961",
						  "OwnerCount": 0,
						  "Sequence": 90659082
						},
						"PreviousTxnID": "D2D2B59405D5220E146CF695572D189BD81AEF3F7724B94FA827CC382DB11675",
						"PreviousTxnLgrSeq": 90659082
					  }
					},
					{
					  "CreatedNode": {
						"LedgerEntryType": "RippleState",
						"LedgerIndex": "43E6E4D1D3A83C5C663B687DE18C69B951E7B474942BB9C82904812DF136E4D8",
						"NewFields": {
						  "Balance": {
							"currency": "USD",
							"issuer": "rrrrrrrrrrrrrrrrrrrrBZbvji",
							"value": "2.6247417128"
						  },
						  "Flags": 1114112,
						  "HighLimit": {
							"currency": "USD",
							"issuer": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
							"value": "0"
						  },
						  "HighNode": "2391",
						  "LowLimit": {
							"currency": "USD",
							"issuer": "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
							"value": "0"
						  }
						}
					  }
					},
					{
					  "CreatedNode": {
						"LedgerEntryType": "DirectoryNode",
						"LedgerIndex": "58E80E7203517DCAB018C65EFC07C84159571D634BECED0A41385FC7490B8788",
						"NewFields": {
						  "Owner": "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
						  "RootIndex": "58E80E7203517DCAB018C65EFC07C84159571D634BECED0A41385FC7490B8788"
						}
					  }
					},
					{
					  "CreatedNode": {
						"LedgerEntryType": "DirectoryNode",
						"LedgerIndex": "658E15E434481B905B7E21515799D0D254A54E9BFA3B7B6837619181E3922FCA",
						"NewFields": {
						  "IndexPrevious": "2390",
						  "Owner": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
						  "RootIndex": "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A"
						}
					  }
					},
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "Flags": 0,
						  "IndexNext": "2391",
						  "IndexPrevious": "238f",
						  "Owner": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
						  "RootIndex": "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A"
						},
						"LedgerEntryType": "DirectoryNode",
						"LedgerIndex": "91762DD13177F60DE0F96944972C4252717A0391C0604F2B6D8BFF89ED8D63D4",
						"PreviousFields": {
						  "IndexNext": "0"
						}
					  }
					},
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "Balance": {
							"currency": "USD",
							"issuer": "rrrrrrrrrrrrrrrrrrrrBZbvji",
							"value": "26765.842495683"
						  },
						  "Flags": 16842752,
						  "HighLimit": {
							"currency": "USD",
							"issuer": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
							"value": "0"
						  },
						  "HighNode": "230c",
						  "LowLimit": {
							"currency": "USD",
							"issuer": "rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q",
							"value": "0"
						  },
						  "LowNode": "0"
						},
						"LedgerEntryType": "RippleState",
						"LedgerIndex": "9AE3CEB5FBC465610CAD1D890BAAD70EB8489A76EE19B6990E7DC0004D7CFD1F",
						"PreviousFields": {
						  "Balance": {
							"currency": "USD",
							"issuer": "rrrrrrrrrrrrrrrrrrrrBZbvji",
							"value": "26768.4672373958"
						  }
						},
						"PreviousTxnID": "FACEF612B2D8EA190AED5A576E9236C76FC19BC791139FC4CD99C1D7246167BF",
						"PreviousTxnLgrSeq": 90659175
					  }
					},
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "AMMID": "630D4F2C7A2F80C4367BAC35219CE2C1274B59330694769A79B0C94A59789AAF",
						  "Account": "rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q",
						  "Balance": "49407458473",
						  "Flags": 26214400,
						  "OwnerCount": 1,
						  "Sequence": 86795329
						},
						"LedgerEntryType": "AccountRoot",
						"LedgerIndex": "A88F25E5AD1D3945FB52291910763E286C55DBE1157E8F19D00F3CA964C6BC45",
						"PreviousFields": {
						  "Balance": "49402596007"
						},
						"PreviousTxnID": "FACEF612B2D8EA190AED5A576E9236C76FC19BC791139FC4CD99C1D7246167BF",
						"PreviousTxnLgrSeq": 90659175
					  }
					},
					{
					  "ModifiedNode": {
						"LedgerEntryType": "AccountRoot",
						"LedgerIndex": "BF1F2A23D614916E3C6ED2DCC389468CFA09045BEDB54C71A05C5E94EA6C6CFE",
						"PreviousTxnID": "23132EB4C93F01A84AD3DD5132FFA0EB1BBD4F7F18C4651A38E1F75998B39D90",
						"PreviousTxnLgrSeq": 90658052
					  }
					},
					{
					  "ModifiedNode": {
						"FinalFields": {
						  "Flags": 0,
						  "IndexNext": "1",
						  "IndexPrevious": "2391",
						  "Owner": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
						  "RootIndex": "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A"
						},
						"LedgerEntryType": "DirectoryNode",
						"LedgerIndex": "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A",
						"PreviousFields": {
						  "IndexPrevious": "2390"
						}
					  }
					}
				  ],
				  "TransactionIndex": 1,
				  "TransactionResult": "tesSUCCESS"
				},
				"validated": true,
				"date": 779303540,
				"ledger_index": 90659219,
				"inLedger": 90659219,
				"status": "success"
			  }
			}`,
			ledgerResp: types.LedgerResponse{
				Result: types.LedgerResult{
					Ledger: types.LedgerInfo{
						Closed:      false,
						LedgerIndex: "91225188",
						ParentHash:  "3E54FF795235548F8B62078F9CE5B5427D7B86BB73571C5CBD9044E171842218",
					},
					LedgerCurrentIndex: 91225188,
					Validated:          false,
					Status:             "success",
				},
			},
			expectedTxInfo: xclient.TxInfo{
				Name:  "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
				Hash:  "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
				Chain: "",
				Block: &xclient.Block{
					Height: 90659219,
					Hash:   "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
				},
				Transfers: []*xclient.Transfer{
					{
						From: []*xclient.BalanceChange{
							{
								Asset:    "chains/assets",
								Contract: "",
								Balance:  xc.NewAmountBlockchainFromStr("4862478000000"),
								Amount:   nil,
								Address:  "chains/addresses/rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
							},
							{
								Asset:    "chains/assets/USD-rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
								Contract: "USD-rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
								Balance:  xc.NewAmountBlockchainFromStr("2624741712800000"),
								Amount:   nil,
								Address:  "chains/addresses/rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q",
							},
						},
						To: []*xclient.BalanceChange{
							{
								Asset:    "chains/assets/USD-rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
								Contract: "USD-rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
								Balance:  xc.NewAmountBlockchainFromStr("2624741712800000"),
								Amount:   nil,
								Address:  "chains/addresses/rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
							},
							{
								Asset:    "chains/assets",
								Contract: "",
								Balance:  xc.NewAmountBlockchainFromStr("4862466000000"),
								Amount:   nil,
								Address:  "chains/addresses/rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q",
							},
						},
					},
				},
				Fees: []*xclient.Balance{
					{
						Asset:   "chains/assets",
						Balance: xc.NewAmountBlockchainFromStr("12000000"),
					},
				},
				Confirmations: 566106,
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
				w.Write([]byte(vector.txResp))
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
			require.Equal(t, vector.expectedTxInfo.Name, txInfo.Name)
			require.Equal(t, vector.expectedTxInfo.Hash, txInfo.Hash)
			require.Equal(t, vector.expectedTxInfo.Chain, txInfo.Chain)
			require.Equal(t, vector.expectedTxInfo.Block.Hash, txInfo.Block.Hash)
			require.Equal(t, vector.expectedTxInfo.Block.Height, txInfo.Block.Height)
			for i := range vector.expectedTxInfo.Transfers {
				require.Equal(t, *vector.expectedTxInfo.Transfers[i], *txInfo.Transfers[i])
			}
			for i := range vector.expectedTxInfo.Fees {
				require.Equal(t, *vector.expectedTxInfo.Fees[i], *txInfo.Fees[i])
			}
			require.Equal(t, vector.expectedTxInfo.Confirmations, txInfo.Confirmations)
			require.Equal(t, vector.expectedTxInfo.Error, txInfo.Error)
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
			types.AccountInfoResponse{
				Result: types.AccountInfoResultDetails{
					AccountData: types.AccountData{
						Balance: "20000000",
					},
				},
			},
			xc.NewAmountBlockchainFromUint64(20000000),
			"",
		},
		{
			types.AccountInfoResponse{},
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
			types.AccountLinesResponse{
				Result: types.AccountLinesResultDetails{
					Lines: []types.Line{
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
			types.AccountLinesResponse{},
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
