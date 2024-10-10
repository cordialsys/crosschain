package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	xrptx "github.com/cordialsys/crosschain/chain/xrp/tx"
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

func TestSubmitTx(t *testing.T) {

	vectors := []struct {
		txInput    xc.Tx
		submitResp xrpClient.SubmitResponse
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
			submitResp: xrpClient.SubmitResponse{
				Result: xrpClient.SubmitResult{
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
		asset      xc.ITask
		txHash     string
		txResp     xrpClient.TransactionResponse
		ledgerResp xrpClient.LedgerResponse
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
					Meta: xrpClient.TransactionMeta{
						AffectedNodes: []xrpClient.AffectedNodes{
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										Account: "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
										Balance: &xrpClient.Balance{
											XRPAmount: "20000000",
										},
										Flags:      0,
										OwnerCount: 0,
										Sequence:   92557,
									},
									LedgerEntryType: "AccountRoot",
									LedgerIndex:     "18F34B88295C6BBD378F0F94E600660C8B7CDEAC89A3C41236910B3334F352FE",
									PreviousFields: &xrpClient.PreviousFields{
										Balance: xrpClient.Balance{
											XRPAmount: "10000000",
										},
									},
									PreviousTxnID:     "364A0DB3EDF04CB1661C29F6120224FB8984FBED51F4DEC431E5D8BEE61BF00F",
									PreviousTxnLgrSeq: 92557,
								},
							},
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										Account: "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
										Balance: &xrpClient.Balance{
											XRPAmount: "79999976",
										},
										Flags:      0,
										OwnerCount: 0,
										Sequence:   92262,
									},
									LedgerEntryType: "AccountRoot",
									LedgerIndex:     "373EBB701A602BFB0D5D1648D7361A3E5D40FD2FD3FA6D5FA9B5CD73E4AE7003",
									PreviousFields: &xrpClient.PreviousFields{
										Balance: xrpClient.Balance{
											XRPAmount: "89999988",
										},
										Sequence: 92261,
									},
									PreviousTxnID:     "364A0DB3EDF04CB1661C29F6120224FB8984FBED51F4DEC431E5D8BEE61BF00F",
									PreviousTxnLgrSeq: 92257,
								},
							},
						},
					},
					Status: "success",
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
			err: "",
		},
		{
			asset:  &xc.ChainConfig{},
			txHash: "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
			txResp: xrpClient.TransactionResponse{
				Result: xrpClient.TransactionResult{
					Account:            "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
					Amount:             "",
					Destination:        "",
					Fee:                "12",
					Flags:              786432,
					LastLedgerSequence: 90659227,
					Sequence:           90659082,
					SigningPubKey:      "03096E30DF354C174D22ACD99C201FCE1CC6EE588D58F11CF858A45FDE4FCF0C6E",
					TransactionType:    "OfferCreate",
					TxnSignature:       "3045022100C77D56EF2F3B4995D9F021D78613490915A5BE8AC3F7BFEE8BEEED3C81B646E40220650629F4D40B3803D14A402926482F042841AA33DB617C6B8CA63DFC85E87188",
					Hash:               "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B",
					DeliverMax:         "",
					TakerGets: &xrpClient.TakeGetsOrPays{
						XRPAmount: "4862466",
					},
					TakerPays: &xrpClient.TakeGetsOrPays{
						TokenAmount: &xrpClient.Amount{
							Currency: "USD",
							Issuer:   "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
							Value:    "2.5",
						},
					},
					Meta: xrpClient.TransactionMeta{
						AffectedNodes: []xrpClient.AffectedNodes{
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										Account: "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
										Balance: &xrpClient.Balance{
											XRPAmount: "32008483",
										},
										Flags:      0,
										OwnerCount: 1,
										Sequence:   90659083,
									},
									LedgerEntryType: "AccountRoot",
									LedgerIndex:     "2CD4DCB5BAE3A17AA69B12101056D4AB5A91269D5A1132DEF611019B9A3E1DC5",
									PreviousFields: &xrpClient.PreviousFields{
										Balance: xrpClient.Balance{
											XRPAmount: "36870961",
										},
										Sequence: 90659082,
									},
									PreviousTxnID:     "D2D2B59405D5220E146CF695572D189BD81AEF3F7724B94FA827CC382DB11675",
									PreviousTxnLgrSeq: 90659082,
								},
							},
							{
								CreatedNode: &xrpClient.CreatedNode{
									LedgerEntryType: "RippleState",
									LedgerIndex:     "43E6E4D1D3A83C5C663B687DE18C69B951E7B474942BB9C82904812DF136E4D8",
									NewFields: xrpClient.NewFields{
										Balance: &xrpClient.Balance{
											TokenAmount: &xrpClient.Amount{
												Currency: "USD",
												Issuer:   "rrrrrrrrrrrrrrrrrrrrBZbvji",
												Value:    "2.6247417128",
											},
										},
										Flags: 1114112,
										HighLimit: &xrpClient.Amount{
											Currency: "USD",
											Issuer:   "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
											Value:    "0",
										},
										HighNode: "2391",
										LowLimit: &xrpClient.Amount{
											Currency: "USD",
											Issuer:   "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
											Value:    "0",
										},
									},
								},
							},
							{
								CreatedNode: &xrpClient.CreatedNode{
									LedgerEntryType: "DirectoryNode",
									LedgerIndex:     "58E80E7203517DCAB018C65EFC07C84159571D634BECED0A41385FC7490B8788",
									NewFields: xrpClient.NewFields{
										Owner:     "rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw",
										RootIndex: "58E80E7203517DCAB018C65EFC07C84159571D634BECED0A41385FC7490B8788",
									},
								},
							},
							{
								CreatedNode: &xrpClient.CreatedNode{
									LedgerEntryType: "DirectoryNode",
									LedgerIndex:     "658E15E434481B905B7E21515799D0D254A54E9BFA3B7B6837619181E3922FCA",
									NewFields: xrpClient.NewFields{
										IndexPrevious: "2390",
										Owner:         "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
										RootIndex:     "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A",
									},
								},
							},
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										Flags:         0,
										IndexNext:     "2391",
										IndexPrevious: "238f",
										Owner:         "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
										RootIndex:     "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A",
									},
									LedgerEntryType: "DirectoryNode",
									LedgerIndex:     "91762DD13177F60DE0F96944972C4252717A0391C0604F2B6D8BFF89ED8D63D4",
									PreviousFields: &xrpClient.PreviousFields{
										IndexNext: "0",
									},
								},
							},
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										Balance: &xrpClient.Balance{
											TokenAmount: &xrpClient.Amount{
												Currency: "USD",
												Issuer:   "rrrrrrrrrrrrrrrrrrrrBZbvji",
												Value:    "26765.842495683",
											},
										},
										Flags: 16842752,
										HighLimit: &xrpClient.Amount{
											Currency: "USD",
											Issuer:   "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
											Value:    "0",
										},
										HighNode: "230c",
										LowLimit: &xrpClient.Amount{
											Currency: "USD",
											Issuer:   "rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q",
											Value:    "0",
										},
										LowNode: "0",
									},
									LedgerEntryType: "RippleState",
									LedgerIndex:     "9AE3CEB5FBC465610CAD1D890BAAD70EB8489A76EE19B6990E7DC0004D7CFD1F",
									PreviousFields: &xrpClient.PreviousFields{
										Balance: xrpClient.Balance{
											TokenAmount: &xrpClient.Amount{
												Currency: "USD",
												Issuer:   "rrrrrrrrrrrrrrrrrrrrBZbvji",
												Value:    "26768.4672373958",
											},
										},
									},
									PreviousTxnID:     "FACEF612B2D8EA190AED5A576E9236C76FC19BC791139FC4CD99C1D7246167BF",
									PreviousTxnLgrSeq: 90659175,
								},
							},
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										AMMID:   "630D4F2C7A2F80C4367BAC35219CE2C1274B59330694769A79B0C94A59789AAF",
										Account: "rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q",
										Balance: &xrpClient.Balance{
											XRPAmount: "49407458473",
										},
										Flags:      26214400,
										OwnerCount: 1,
										Sequence:   86795329,
									},
									LedgerEntryType: "AccountRoot",
									LedgerIndex:     "A88F25E5AD1D3945FB52291910763E286C55DBE1157E8F19D00F3CA964C6BC45",
									PreviousFields: &xrpClient.PreviousFields{
										Balance: xrpClient.Balance{
											XRPAmount: "49402596007",
										},
									},
									PreviousTxnID:     "FACEF612B2D8EA190AED5A576E9236C76FC19BC791139FC4CD99C1D7246167BF",
									PreviousTxnLgrSeq: 90659175,
								},
							},
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									LedgerEntryType:   "AccountRoot",
									LedgerIndex:       "BF1F2A23D614916E3C6ED2DCC389468CFA09045BEDB54C71A05C5E94EA6C6CFE",
									PreviousTxnID:     "23132EB4C93F01A84AD3DD5132FFA0EB1BBD4F7F18C4651A38E1F75998B39D90",
									PreviousTxnLgrSeq: 90658052,
								},
							},
							{
								ModifiedNode: &xrpClient.ModifiedNode{
									FinalFields: &xrpClient.FinalFields{
										Flags:         0,
										IndexNext:     "1",
										IndexPrevious: "2391",
										Owner:         "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
										RootIndex:     "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A",
									},
									LedgerEntryType: "DirectoryNode",
									LedgerIndex:     "D7AC7D74720E29A563100F2B494BADB198F8A9E9FA46F57AE07123151E0DFA7A",
									PreviousFields: &xrpClient.PreviousFields{
										IndexPrevious: "2390",
									},
								},
							},
						},
						TransactionIndex:  1,
						TransactionResult: "tesSUCCESS",
					},
					CtID:        "C567599300010000",
					Validated:   true,
					Date:        779303540,
					LedgerIndex: 90659219,
					InLedger:    90659219,
					Status:      "success",
				},
			},
			ledgerResp: xrpClient.LedgerResponse{
				Result: xrpClient.LedgerResult{
					Ledger: xrpClient.LedgerInfo{
						Closed:      false,
						LedgerIndex: "91225188",
						ParentHash:  "3E54FF795235548F8B62078F9CE5B5427D7B86BB73571C5CBD9044E171842218",
					},
					LedgerCurrentIndex: 91225188,
					Validated:          false,
					Status:             "success",
				},
			},
			err: "",
		},
	}

	for testNo, vector := range vectors {
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
			if testNo == 0 {
				require.NoError(t, err)
				require.NotNil(t, txInfo)

				require.Equal(t, txInfo.Hash, "3F27C0AF1993AF63E3438BA903B981AA095B6C81AB23976A9729B44AB39719BA")

				require.Contains(t, txInfo.Transfers[0].From[0].Address, "chains/addresses/rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S")
				require.Equal(t, txInfo.Transfers[0].From[0].Balance.String(), "10000012")
				require.Contains(t, txInfo.Transfers[0].To[0].Address, "chains/addresses/rLETt614usCXtkc8YcQmrzachrCaDjACjP")
				require.Equal(t, txInfo.Transfers[0].To[0].Balance.String(), "10000000")

				require.Equal(t, txInfo.Fees[0].Balance.String(), "12")

			} else if testNo == 1 {
				require.NoError(t, err)
				require.NotNil(t, txInfo)

				require.Equal(t, txInfo.Hash, "9D4D9CB01F4FFB12CA6262966311936B182E325A80461645E78EF54C11D2751B")

				require.Contains(t, txInfo.Transfers[0].From[0].Address, "chains/addresses/rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw")
				require.Equal(t, txInfo.Transfers[0].From[0].Balance.String(), "4862478")
				require.Contains(t, txInfo.Transfers[0].From[1].Address, "chains/addresses/rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq")
				require.Contains(t, txInfo.Transfers[0].From[1].Asset, "chains/assets/rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq")
				require.Contains(t, txInfo.Transfers[0].From[1].Contract, "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq")
				require.Equal(t, txInfo.Transfers[0].From[1].Balance.String(), "2624741712799732")

				require.Contains(t, txInfo.Transfers[0].To[0].Address, "chains/addresses/rzvAXDKJnPi8m25HjXYiXAjJnzc7LGTfw")
				require.Contains(t, txInfo.Transfers[0].To[0].Asset, "chains/assets/rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq")
				require.Contains(t, txInfo.Transfers[0].To[0].Contract, "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq")
				require.Equal(t, txInfo.Transfers[0].To[0].Balance.String(), "2624741712800000")
				require.Contains(t, txInfo.Transfers[0].To[1].Address, "chains/addresses/rs9ineLqrCzeAGS1bxsrW8x2n3bRJYAh3Q")
				require.Equal(t, txInfo.Transfers[0].To[1].Balance.String(), "4862466")

				require.Equal(t, txInfo.Fees[0].Balance.String(), "12")
				require.Equal(t, txInfo.Fees[1].Balance.String(), "-268")
			}

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
