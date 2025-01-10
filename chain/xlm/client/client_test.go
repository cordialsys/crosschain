package client_test

import (
	"context"
	"encoding/json"

	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	client "github.com/cordialsys/crosschain/chain/xlm/client"
	"github.com/cordialsys/crosschain/chain/xlm/client/types"
	tx "github.com/cordialsys/crosschain/chain/xlm/tx"
	txinput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go/xdr"

	xclient "github.com/cordialsys/crosschain/client"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	defaultClient, err := client.NewClient(&xc.ChainConfig{})
	require.Nil(t, defaultClient)
	require.Error(t, err)

	badConfig := &xc.ChainConfig{
		ChainIDStr:           "Some chain id",
		ChainGasPriceDefault: -1,
	}
	badClient, err := client.NewClient(badConfig)
	require.Nil(t, badClient)
	require.ErrorContains(t, err, "ChainGasPriceDefault cannot be negative")

	badConfig = &xc.ChainConfig{
		ChainIDStr:       "Some chain id",
		ChainMinGasPrice: -1,
	}
	badClient, err = client.NewClient(badConfig)
	require.Nil(t, badClient)
	require.ErrorContains(t, err, "ChainMinGasPrice cannot be negative")

	config := &xc.ChainConfig{
		ChainIDStr: "ChainID",
	}
	client, err := client.NewClient(config)
	require.NotNil(t, client)
	require.NoError(t, err)
}

func TestFetchTxInput(t *testing.T) {
	txActiveTime, err := time.ParseDuration("2h")
	require.NoError(t, err)

	vectors := []struct {
		asset                 xc.ITask
		getAccountResult      types.GetAccountResult
		getLatestLedgerResult types.GetLatestLedgerResult
		err                   string
		expectedTxInput       txinput.TxInput
	}{
		{
			asset: &xc.ChainConfig{
				ChainGasPriceDefault:  0.00001,
				TransactionActiveTime: txActiveTime,
				ChainIDStr:            "Test SDF Network ; September 2015",
			},
			getAccountResult: types.GetAccountResult{
				Sequence: "1212",
			},
			getLatestLedgerResult: types.GetLatestLedgerResult{
				Embedded: types.Records{
					Records: []types.GetLedgerResult{
						{
							Id:       "5",
							Hash:     "totally_valid_hash",
							Sequence: 1111,
						},
					},
				},
			},
			err: "",
			expectedTxInput: txinput.TxInput{
				TxInputEnvelope: xc.TxInputEnvelope{Type: "xlm"},
				Sequence:        1213,
				BaseFee:         100,
				MaxFee:          100,
				// 2h * nanoseconds (10^9)
				TransactionActiveTime: time.Duration(7200 * 1e9),
				MinLedgerSequence:     1111,
				Passphrase:            "Test SDF Network ; September 2015",
			},
		},
	}

	for i, vector := range vectors {
		fmt.Printf("\nRunning TestFetchTxInput-%d\n", i)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL.String()
			if strings.Contains(url, "/accounts/") {
				json.NewEncoder(w).Encode(vector.getAccountResult)
			} else if strings.Contains(url, "/ledgers") {
				json.NewEncoder(w).Encode(vector.getLatestLedgerResult)
			} else {
				t.Errorf("unexpected url: %s", url)
			}
		}))
		defer server.Close()

		if _, ok := vector.asset.(*xc.TokenAssetConfig); ok {
			// TODO: Implement when tokens are finished
		} else {
			vector.asset.(*xc.ChainConfig).URL = server.URL
			vector.asset.(*xc.ChainConfig).Decimals = 7
		}

		client, _ := client.NewClient(vector.asset)
		from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
		to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
		input, err := client.FetchLegacyTxInput(
			context.Background(),
			from,
			to,
		)
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
	from := xc.Address("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF")
	var source xdr.MuxedAccount
	err := source.SetAddress(string(from))
	require.NoError(t, err)

	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	var destination xdr.MuxedAccount
	err = destination.SetAddress(string(to))
	require.NoError(t, err)

	require.NoError(t, err)
	preconditions := tx.Preconditions{
		TimeBounds: tx.NewInfiniteTimeout(),
	}

	vectors := []struct {
		txInput xc.Tx
		error   string
	}{
		{
			txInput: &tx.Tx{
				TxEnvelope: &xdr.TransactionEnvelope{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: source,
							Fee:           xdr.Uint32(100),
							SeqNum:        338194314821647,
							Cond:          preconditions.BuildXDR(),
							Operations: []xdr.Operation{
								{
									SourceAccount: &source,
									Body: xdr.OperationBody{
										Type: xdr.OperationTypePayment,
										PaymentOp: &xdr.PaymentOp{
											Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
											Destination: destination,
											Amount:      xdr.Int64(10000000),
										},
									},
								},
							},
						},
						Signatures: []xdr.DecoratedSignature{
							{
								Hint:      [4]byte{57, 234, 167, 44},
								Signature: []byte{76, 51, 186, 154, 227, 143, 149, 39, 183, 152, 173, 83, 50, 63, 221, 130, 197, 118, 246, 30, 240, 3, 76, 48, 214, 166, 72, 248, 30, 98, 172, 161, 138, 67, 145, 48, 26, 55, 132, 10, 66, 22, 68, 119, 3, 77, 57, 31, 236, 107, 181, 221, 226, 227, 161, 248, 59, 232, 44, 127, 126, 237, 215, 5},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{76, 51, 186, 154, 227, 143, 149, 39, 183, 152, 173, 83, 50, 63, 221, 130, 197, 118, 246, 30, 240, 3, 76, 48, 214, 166, 72, 248, 30, 98, 172, 161, 138, 67, 145, 48, 26, 55, 132, 10, 66, 22, 68, 119, 3, 77, 57, 31, 236, 107, 181, 221, 226, 227, 161, 248, 59, 232, 44, 127, 126, 237, 215, 5},
				},
			},
		},
		{
			txInput: &tx.Tx{
				TxEnvelope: &xdr.TransactionEnvelope{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: source,
							Fee:           xdr.Uint32(100),
							SeqNum:        338194314821647,
							Cond:          preconditions.BuildXDR(),
							Operations:    []xdr.Operation{},
						},
						Signatures: []xdr.DecoratedSignature{
							{
								Hint:      [4]byte{57, 234, 167, 44},
								Signature: []byte{76, 51, 186, 154, 227, 143, 149, 39, 183, 152, 173, 83, 50, 63, 221, 130, 197, 118, 246, 30, 240, 3, 76, 48, 214, 166, 72, 248, 30, 98, 172, 161, 138, 67, 145, 48, 26, 55, 132, 10, 66, 22, 68, 119, 3, 77, 57, 31, 236, 107, 181, 221, 226, 227, 161, 248, 59, 232, 44, 127, 126, 237, 215, 5},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{76, 51, 186, 154, 227, 143, 149, 39, 183, 152, 173, 83, 50, 63, 221, 130, 197, 118, 246, 30, 240, 3, 76, 48, 214, 166, 72, 248, 30, 98, 172, 161, 138, 67, 145, 48, 26, 55, 132, 10, 66, 22, 68, 119, 3, 77, 57, 31, 236, 107, 181, 221, 226, 227, 161, 248, 59, 232, 44, 127, 126, 237, 215, 5},
				},
			},
			error: "missing transaction operations",
		},
		{
			txInput: &tx.Tx{
				TxEnvelope: &xdr.TransactionEnvelope{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: source,
							Fee:           xdr.Uint32(100),
							SeqNum:        338194314821647,
							Cond:          preconditions.BuildXDR(),
							Operations: []xdr.Operation{
								{
									SourceAccount: &source,
									Body: xdr.OperationBody{
										Type: xdr.OperationTypePayment,
										PaymentOp: &xdr.PaymentOp{
											Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
											Destination: destination,
											Amount:      xdr.Int64(10000000),
										},
									},
								},
							},
						},
					},
				},
			},
			error: "missing transaction signatures",
		},
	}

	for _, vector := range vectors {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode("")
		}))
		defer server.Close()

		txActiveTime, err := time.ParseDuration("2h")
		require.NoError(t, err)
		client, _ := client.NewClient(&xc.ChainConfig{
			URL:                   server.URL,
			Decimals:              7,
			TransactionActiveTime: txActiveTime,
			ChainGasPriceDefault:  0.00001,
			ChainIDStr:            "Test SDF Network ; September 2015",
		})

		err = client.SubmitTx(context.Background(), vector.txInput)
		if err != nil {
			require.ErrorContains(t, err, vector.error)
		} else {
			require.NoError(t, err)
		}
	}
}

func MustParseTime(s string) time.Time {
	time, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("failed to parse time")
	}
	return time
}

func NewMovement(
	chain string,
	contract string,
	from []*xclient.BalanceChange,
	to []*xclient.BalanceChange,
) *xclient.Movement {
	movement := xclient.NewMovement(xc.NativeAsset(chain), xc.ContractAddress(contract))
	movement.From = from
	movement.To = to

	return movement
}

func TestFetchTxInfo(t *testing.T) {
	vectors := []struct {
		hash                  string
		getTxResult           string
		getLedgerResult       string
		getLatestLedgerResult string
		expected              xclient.TxInfo
		err                   string
	}{
		{
			hash: "1ea250b425a38ed08bab46a141b467e19a2789548c36abc0f4ae3e363fd8a1f3",
			getTxResult: `{
				"id": "1ea250b425a38ed08bab46a141b467e19a2789548c36abc0f4ae3e363fd8a1f3",
				"paging_token": "2210293249740800",
				"successful": true,
				"hash": "1ea250b425a38ed08bab46a141b467e19a2789548c36abc0f4ae3e363fd8a1f3",
				"ledger": 514624,
				"created_at": "2025-01-09T12:46:09Z",
				"source_account": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
				"source_account_sequence": "338194314821647",
				"fee_account": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
				"fee_charged": "100",
				"max_fee": "100",
				"operation_count": 1,
				"envelope_xdr": "AAAAAgAAAADW7ZHzNlAv9wbZdynXq1Uh4jDDk1PKeTctKx/COeqnLAAAAGQAATOWAAAADwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAANbtkfM2UC/3Btl3KderVSHiMMOTU8p5Ny0rH8I56qcsAAAAAQAAAACRNTzkRh4f82hnB4DraHZREIIf8n0Gqe+9Uab3/U53nwAAAAAAAAAAAJiWgAAAAAAAAAABOeqnLAAAAEBMM7qa44+VJ7eYrVMyP92CxXb2HvADTDDWpkj4HmKsoYpDkTAaN4QKQhZEdwNNOR/sa7Xd4uOh+DvoLH9+7dcF",
				"result_xdr": "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA=",
				"memo_type": "none",
				"signatures": [
					"TDO6muOPlSe3mK1TMj/dgsV29h7wA0ww1qZI+B5irKGKQ5EwGjeECkIWRHcDTTkf7Gu13eLjofg76Cx/fu3XBQ=="
				],
				"preconditions": {
					"timebounds": {
						"min_time": "0"
					}
				}
			}`,
			getLedgerResult: `{
				"id": "e7c43d43c778e6e4d3503c59f8226a4f0af36a1ea89b7643c1a77c4296e02d0f",
				"paging_token": "2210293249736704",
				"hash": "e7c43d43c778e6e4d3503c59f8226a4f0af36a1ea89b7643c1a77c4296e02d0f",
				"prev_hash": "c616391d7034f5d4ccfd203b1ecda2fba9b1a449bd9566a6eeea4875263cbfa3",
				"sequence": 514624,
				"successful_transaction_count": 2,
				"failed_transaction_count": 1,
				"operation_count": 2,
				"tx_set_operation_count": 3,
				"closed_at": "2025-01-09T12:46:09Z",
				"total_coins": "100000000000.0000000",
				"fee_pool": "16753.9196547",
				"base_fee_in_stroops": 100,
				"base_reserve_in_stroops": 5000000,
				"max_tx_set_size": 200,
				"protocol_version": 22
			}`,
			getLatestLedgerResult: `{
				"_embedded": {
					"records": [
						{
							"id": "dbaea72b5a49689c63df5afa0cef9ac40cda1174e9c06612dd8c4d741665c726",
							"paging_token": "2234821807964160",
							"hash": "dbaea72b5a49689c63df5afa0cef9ac40cda1174e9c06612dd8c4d741665c726",
							"prev_hash": "9dffc1f46afdaeae61cbab8f38d4142886b9c26bb5be0127b14643e04e88f46d",
							"sequence": 520335,
							"successful_transaction_count": 2,
							"failed_transaction_count": 4,
							"operation_count": 3,
							"tx_set_operation_count": 10,
							"closed_at": "2025-01-09T20:42:33Z",
							"total_coins": "100000000000.0000000",
							"fee_pool": "16974.5816010",
							"base_fee_in_stroops": 100,
							"base_reserve_in_stroops": 5000000,
							"max_tx_set_size": 200,
							"protocol_version": 22
						}
					]
				}
			}`,
			expected: xclient.TxInfo{
				Name:   "chains/XLM/transactions/1ea250b425a38ed08bab46a141b467e19a2789548c36abc0f4ae3e363fd8a1f3",
				Hash:   "1ea250b425a38ed08bab46a141b467e19a2789548c36abc0f4ae3e363fd8a1f3",
				XChain: xc.NativeAsset("XLM"),
				Block: &xclient.Block{
					Chain:  xc.NativeAsset("XLM"),
					Height: 514624,
					Hash:   "e7c43d43c778e6e4d3503c59f8226a4f0af36a1ea89b7643c1a77c4296e02d0f",
					Time:   MustParseTime("2025-01-09T12:46:09Z"),
				},
				Movements: []*xclient.Movement{
					NewMovement(
						"XLM",
						"XLM",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(10000000),
								XAddress:  "chains/XLM/addresses/GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
								AddressId: "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
							},
						},
						[]*xclient.BalanceChange{{
							Balance:   xc.NewAmountBlockchainFromUint64(10000000),
							XAddress:  "chains/XLM/addresses/GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI",
							AddressId: "GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI",
						}},
					),
					NewMovement(
						"XLM",
						"XLM",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100),
								XAddress:  "chains/XLM/addresses/GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
								AddressId: "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
							},
						},
						[]*xclient.BalanceChange{},
					),
				},
				Fees: []*xclient.Balance{
					{
						Asset:    "chains/XLM/assets/XLM",
						Contract: "XLM",
						Balance:  xc.NewAmountBlockchainFromUint64(100),
					},
				},
				Confirmations: 5711,
			},
		},
	}

	for _, vector := range vectors {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL.String()

			if strings.Contains(url, "/transactions/") {
				w.Write([]byte(vector.getTxResult))
			} else if strings.Contains(url, "/ledgers/") {
				w.Write([]byte(vector.getLedgerResult))
			} else if strings.Contains(url, "/ledgers?order=desc&limit=1") {
				w.Write([]byte(vector.getLatestLedgerResult))
			} else {
				t.Errorf("unexpected url: %s", url)
			}
		}))
		defer server.Close()

		client, _ := client.NewClient(&xc.ChainConfig{
			Chain:    "XLM",
			URL:      server.URL,
			Decimals: 7,
			ChainIDStr: "Test SDF Network ; September 2015",
		})
		txInfo, err := client.FetchTxInfo(context.Background(), xc.TxHash(vector.hash))
		if vector.err != "" {
			require.Equal(t, xclient.TxInfo{}, txInfo)
			require.ErrorContains(t, err, vector.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, txInfo)
			require.Equal(t, vector.expected, txInfo)
		}
	}
}

func TestFetchNativeBalance(t *testing.T) {
	vectors := []struct {
		getAccountResult string
		expected         xc.AmountBlockchain
		err              string
	}{
		{
			getAccountResult: `{
			  "id": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
			  "account_id": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
			  "sequence": "338194314821647",
			  "sequence_ledger": 514624,
			  "sequence_time": "1736426769",
			  "subentry_count": 0,
			  "last_modified_ledger": 514624,
			  "last_modified_time": "2025-01-09T12:46:09Z",
			  "thresholds": {
				"low_threshold": 0,
				"med_threshold": 0,
				"high_threshold": 0
			  },
			  "flags": {
				"auth_required": false,
				"auth_revocable": false,
				"auth_immutable": false,
				"auth_clawback_enabled": false
			  },
			  "balances": [
				{
				  "balance": "9989.9998500",
				  "buying_liabilities": "0.0000000",
				  "selling_liabilities": "0.0000000",
				  "asset_type": "native"
				}
			  ],
			  "signers": [
				{
				  "weight": 1,
				  "key": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
				  "type": "ed25519_public_key"
				}
			  ],
			  "data": {},
			  "num_sponsoring": 0,
			  "num_sponsored": 0,
			  "paging_token": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF"
			}`,
			expected: xc.NewAmountBlockchainFromUint64(99899998500),
		},
	}

	for _, vector := range vectors {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL.String()
			if strings.Contains(url, "/accounts/") {
				w.Write([]byte(vector.getAccountResult))
			} else {
				t.Errorf("unexpected url: %s", url)
			}
		}))
		defer server.Close()

		client, _ := client.NewClient(&xc.ChainConfig{
			Chain:    "XLM",
			URL:      server.URL,
			Decimals: 7,
			ChainIDStr: "Test SDF Network ; September 2015",
		})

		balance, err := client.FetchBalance(context.Background(), xc.Address("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF"))
		if vector.err != "" {
			require.ErrorContains(t, err, vector.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, balance)
			require.Equal(t, vector.expected, balance)
		}
	}

}
