package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xlm"
	client "github.com/cordialsys/crosschain/chain/xlm/client"
	"github.com/cordialsys/crosschain/chain/xlm/client/types"
	"github.com/cordialsys/crosschain/chain/xlm/common"
	tx "github.com/cordialsys/crosschain/chain/xlm/tx"
	txinput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/cordialsys/crosschain/factory/defaults/chains"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/require"
)

func TestValidClientConfiguration(t *testing.T) {
	xc.NewAmountHumanReadableFromFloat(2.0)
	chain := xc.NewChainConfig(xc.XLM).
		WithChainID("ChainID").
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(2.0)).
		WithTransactionActiveTime(time.Duration(500))

	client, err := client.NewClient(chain)
	require.NotNil(t, client)
	require.NoError(t, err)
}

func TestEmptyClientConfiguration(t *testing.T) {
	defaultClient, err := client.NewClient(xc.NewChainConfig(""))
	require.Nil(t, defaultClient)
	require.Error(t, err)
}

func TestClientConfigurationMissingChainIDStr(t *testing.T) {
	missingChainIDStr := xc.NewChainConfig(xc.XLM).
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(2.0)).
		WithTransactionActiveTime(time.Duration(500))

	badClient, err := client.NewClient(missingChainIDStr)
	require.Nil(t, badClient)
	require.ErrorContains(t, err, "chain-id")
}

func TestClientConfigurationBadMaxFee(t *testing.T) {
	negativeMaxFee := xc.NewChainConfig(xc.XLM).
		WithChainID("Some chain id").
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(-1.1)).
		WithTransactionActiveTime(time.Duration(500))

	badClient, err := client.NewClient(negativeMaxFee)
	require.Nil(t, badClient)
	require.ErrorContains(t, err, "gas-budget-default")

	zeroMaxFee := xc.NewChainConfig(xc.XLM).
		WithChainID("ChainID").
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(0.0)).
		WithTransactionActiveTime(time.Duration(500))
	client, err := client.NewClient(zeroMaxFee)
	require.Nil(t, client)
	require.ErrorContains(t, err, "chain gas-budget-default")
}

func TestClientConfigurationBadTransactionActiveTime(t *testing.T) {
	missingTransactionActiveTime := xc.NewChainConfig(xc.XLM).
		WithChainID("Some chain id").
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(2.0))
	badClient, err := client.NewClient(missingTransactionActiveTime)
	require.Nil(t, badClient)
	require.ErrorContains(t, err, "transaction-active-time")
}

func TestMainnetConfiguration(t *testing.T) {
	mainnetConfig := chains.Mainnet["xlm"]
	require.Greater(t, mainnetConfig.GasBudgetDefault.Decimal().InexactFloat64(), 0.0)
	require.NotZero(t, mainnetConfig.ChainID)
	require.NotZero(t, mainnetConfig.TransactionActiveTime)
}

func TestTestnetConfiguration(t *testing.T) {
	testnetConfig := chains.Testnet["xlm"]
	require.Greater(t, testnetConfig.GasBudgetDefault.Decimal().InexactFloat64(), 0.0)
	require.NotZero(t, testnetConfig.TransactionActiveTime)
}

func TestFetchTxInput(t *testing.T) {
	txActiveTime, err := time.ParseDuration("2h")
	require.NoError(t, err)

	vectors := []struct {
		name                  string
		asset                 *xc.ChainConfig
		contract              xc.ContractAddress
		amount                xc.AmountBlockchain
		getAccountResult      types.GetAccountResult
		getLatestLedgerResult types.GetLatestLedgerResult
		err                   string
		expectedTxInput       txinput.TxInput
	}{
		{
			name: "Test valid Tx input",
			asset: xc.NewChainConfig(xc.XLM).
				WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(0.00001)).
				WithTransactionActiveTime(txActiveTime).
				WithChainID("Test SDF Network ; September 2015"),
			amount: xc.NewAmountBlockchainFromUint64(100),
			getAccountResult: types.GetAccountResult{
				Sequence: "1212",
				Balances: []types.Balance{
					{
						Balance:   "2.0",
						AssetType: "native",
					},
				},
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
				MaxFee:          100,
				// 2h * nanoseconds (10^9)
				TransactionActiveTime: time.Duration(7200 * 1e9),
				MinLedgerSequence:     1111,
				Passphrase:            "Test SDF Network ; September 2015",
			},
		},
		{
			name: "Check fee greater than balance",
			asset: xc.NewChainConfig(xc.XLM).
				WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(5.00000)).
				WithTransactionActiveTime(txActiveTime).
				WithChainID("Test SDF Network ; September 2015"),
			amount: xc.NewAmountBlockchainFromUint64(100),
			getAccountResult: types.GetAccountResult{
				Sequence: "1212",
				Balances: []types.Balance{
					{
						Balance:   "2.0",
						AssetType: "native",
					},
				},
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
				// Balance*10^7 - Amount
				MaxFee: 20000000 - 100,
				// 2h * nanoseconds (10^9)
				TransactionActiveTime: time.Duration(7200 * 1e9),
				MinLedgerSequence:     1111,
				Passphrase:            "Test SDF Network ; September 2015",
			},
		},
		{
			name: "Check balance lower than tx amount",
			asset: xc.NewChainConfig(xc.XLM).
				WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(1.00000)).
				WithTransactionActiveTime(txActiveTime).
				WithChainID("Test SDF Network ; September 2015"),
			amount: xc.NewAmountBlockchainFromUint64(50000000),
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
			err:             "failed to create tx input",
			expectedTxInput: txinput.TxInput{},
		},
		{
			name: "Check fee greater token tx",
			asset: xc.NewChainConfig(xc.XLM).
				WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(5.00000)).
				WithTransactionActiveTime(txActiveTime).
				WithChainID("Test SDF Network ; September 2015"),
			contract: xc.ContractAddress("USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
			amount:   xc.NewAmountBlockchainFromUint64(100),
			getAccountResult: types.GetAccountResult{
				Sequence: "1212",
				Balances: []types.Balance{
					{
						Balance:   "2.0",
						AssetType: "native",
					},
				},
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
				// Balance*10^7
				MaxFee: 20000000,
				// 2h * nanoseconds (10^9)
				TransactionActiveTime: time.Duration(7200 * 1e9),
				MinLedgerSequence:     1111,
				Passphrase:            "Test SDF Network ; September 2015",
			},
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
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

			vector.asset.URL = server.URL
			vector.asset.Decimals = 7

			client, _ := client.NewClient(vector.asset)
			from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
			to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
			args, _ := builder.NewTransferArgs(from, to, vector.amount)
			if vector.contract != "" {
				args.SetContract(vector.contract)
			}
			input, err := client.FetchTransferInput(
				context.Background(),
				args,
			)
			if err != nil {
				t.Logf("Error: %v", err)
				require.Nil(t, input)
				require.ErrorContains(t, err, vector.err)
				require.NotEqual(t, vector.err, "")
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				txInput := input.(xc.TxInput)
				require.Equal(t, &vector.expectedTxInput, txInput)
			}
		})
	}
}

func TestSubmitTx(t *testing.T) {
	from := xc.Address("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF")
	source := common.MustMuxedAccountFromAddres(from)

	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	destination := common.MustMuxedAccountFromAddres(to)

	preconditions := xlm.Preconditions{
		TimeBounds: xlm.NewInfiniteTimeout(),
	}

	vectors := []struct {
		name     string
		txInput  xc.Tx
		response string
		error    string
	}{
		{
			name: "Test native transaction",
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
								Hint: [4]byte{57, 234, 167, 44},
								Signature: []byte{
									76, 51, 186, 154, 227, 143,
									149, 39, 183, 152, 173, 83,
									50, 63, 221, 130, 197, 118,
									246, 30, 240, 3, 76, 48,
									214, 166, 72, 248, 30, 98,
									172, 161, 138, 67, 145, 48,
									26, 55, 132, 10, 66, 22,
									68, 119, 3, 77, 57, 31,
									236, 107, 181, 221, 226, 227,
									161, 248, 59, 232, 44, 127,
									126, 237, 215, 5,
								},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{
						76, 51, 186, 154, 227, 143,
						149, 39, 183, 152, 173, 83,
						50, 63, 221, 130, 197, 118,
						246, 30, 240, 3, 76, 48,
						214, 166, 72, 248, 30, 98,
						172, 161, 138, 67, 145, 48,
						26, 55, 132, 10, 66, 22,
						68, 119, 3, 77, 57, 31,
						236, 107, 181, 221, 226, 227,
						161, 248, 59, 232, 44, 127,
						126, 237, 215, 5,
					},
				},
			},
			response: `{ "tx_status": "PENDING", "hash": "6cbb7f714bd08cea7c30cab7818a35c510cbbfc0a6aa06172a1e94146ecf0165" }`,
		},
		{
			name: "Test token transaction",
			txInput: &tx.Tx{
				TxEnvelope: &xdr.TransactionEnvelope{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: source,
							Fee:           xdr.Uint32(100),
							SeqNum:        338194314821653,
							Cond:          preconditions.BuildXDR(),
							Operations: []xdr.Operation{
								{
									SourceAccount: &source,
									Body: xdr.OperationBody{
										Type: xdr.OperationTypePayment,
										PaymentOp: &xdr.PaymentOp{
											Asset: xdr.Asset{
												Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
												AlphaNum4: &xdr.AlphaNum4{
													AssetCode: [4]byte{byte('U'), byte('S'), byte('D'), byte('C')},
													Issuer:    common.MustMuxedAccountFromAddres(xc.Address("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")).ToAccountId(),
												},
											},
											Destination: destination,
											Amount:      xdr.Int64(10000000),
										},
									},
								},
							},
						},
						Signatures: []xdr.DecoratedSignature{
							{
								Hint: [4]byte{57, 234, 167, 44},
								Signature: []byte{
									0x64, 0x65, 0x35, 0x34, 0x31, 0x30, 0x37, 0x32, 0x31, 0x34, 0x38, 0x31,
									0x66, 0x33, 0x39, 0x33, 0x62, 0x62, 0x62, 0x34, 0x37, 0x31, 0x37, 0x35,
									0x32, 0x37, 0x39, 0x31, 0x35, 0x62, 0x64, 0x61, 0x33, 0x32, 0x31, 0x64,
									0x64, 0x33, 0x39, 0x37, 0x33, 0x39, 0x39, 0x30, 0x63, 0x65, 0x65, 0x65,
									0x38, 0x65, 0x61, 0x61, 0x38, 0x31, 0x64, 0x33, 0x64, 0x64, 0x33, 0x66,
									0x61, 0x39, 0x34, 0x31, 0x66, 0x61, 0x39, 0x63, 0x37, 0x36, 0x34, 0x30,
									0x37, 0x35, 0x35, 0x66, 0x64, 0x66, 0x38, 0x36, 0x65, 0x62, 0x31, 0x35,
									0x31, 0x36, 0x64, 0x35, 0x34, 0x35, 0x38, 0x64, 0x38, 0x34, 0x65, 0x64,
									0x30, 0x62, 0x63, 0x34, 0x39, 0x64, 0x64, 0x37, 0x66, 0x64, 0x36, 0x34,
									0x62, 0x32, 0x65, 0x61, 0x35, 0x32, 0x33, 0x62, 0x61, 0x65, 0x30, 0x30,
									0x35, 0x61, 0x39, 0x63, 0x65, 0x61, 0x30, 0x34, 0x0a,
								},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{
						76, 51, 186, 154, 227, 143,
						149, 39, 183, 152, 173, 83,
						50, 63, 221, 130, 197, 118,
						246, 30, 240, 3, 76, 48,
						214, 166, 72, 248, 30, 98,
						172, 161, 138, 67, 145, 48,
						26, 55, 132, 10, 66, 22,
						68, 119, 3, 77, 57, 31,
						236, 107, 181, 221, 226, 227,
						161, 248, 59, 232, 44, 127,
						126, 237, 215, 5,
					},
				},
			},
			response: `{ "tx_status": "PENDING", "hash": "6cbb7f714bd08cea7c30cab7818a35c510cbbfc0a6aa06172a1e94146ecf0165" }`,
			error:    "",
		},
		{
			name: "Test AsyncTxSubmissionResponse error reply",
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
								Hint: [4]byte{57, 234, 167, 44},
								Signature: []byte{
									76, 51, 186, 154, 227, 143,
									149, 39, 183, 152, 173, 83,
									50, 63, 221, 130, 197, 118,
									246, 30, 240, 3, 76, 48,
									214, 166, 72, 248, 30, 98,
									172, 161, 138, 67, 145, 48,
									26, 55, 132, 10, 66, 22,
									68, 119, 3, 77, 57, 31,
									236, 107, 181, 221, 226, 227,
									161, 248, 59, 232, 44, 127,
									126, 237, 215, 5,
								},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{
						76, 51, 186, 154, 227, 143,
						149, 39, 183, 152, 173, 83,
						50, 63, 221, 130, 197, 118,
						246, 30, 240, 3, 76, 48,
						214, 166, 72, 248, 30, 98,
						172, 161, 138, 67, 145, 48,
						26, 55, 132, 10, 66, 22,
						68, 119, 3, 77, 57, 31,
						236, 107, 181, 221, 226, 227,
						161, 248, 59, 232, 44, 127,
						126, 237, 215, 5,
					},
				},
			},
			response: `{
				"tx_status": "ERROR",
				"errorResultXDR": "AAAAAAAAAGT////7AAAAAA=="
			}`,
			error: "tx_bad_seq",
		},
		{
			name: "Test AsyncTxSubmissionProblem  reply",
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
								Hint: [4]byte{57, 234, 167, 44},
								Signature: []byte{
									76, 51, 186, 154, 227, 143,
									149, 39, 183, 152, 173, 83,
									50, 63, 221, 130, 197, 118,
									246, 30, 240, 3, 76, 48,
									214, 166, 72, 248, 30, 98,
									172, 161, 138, 67, 145, 48,
									26, 55, 132, 10, 66, 22,
									68, 119, 3, 77, 57, 31,
									236, 107, 181, 221, 226, 227,
									161, 248, 59, 232, 44, 127,
									126, 237, 215, 5,
								},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{
						76, 51, 186, 154, 227, 143,
						149, 39, 183, 152, 173, 83,
						50, 63, 221, 130, 197, 118,
						246, 30, 240, 3, 76, 48,
						214, 166, 72, 248, 30, 98,
						172, 161, 138, 67, 145, 48,
						26, 55, 132, 10, 66, 22,
						68, 119, 3, 77, 57, 31,
						236, 107, 181, 221, 226, 227,
						161, 248, 59, 232, 44, 127,
						126, 237, 215, 5,
					},
				},
			},
			response: `{
				"type": "transaction_malformed",
				"title": "Transaction Malformed",
				"status": 400,
				"detail": "Horizon could not decode the transaction envelope in this request. A transaction should be an XDR TransactionEnvelope struct encoded using base64. The envelope read from this request is echoed in the extras.envelope_xdr field of this response for your convenience.",
				"extras": {
					"envelope_xdr": ""
				}
			}`,
			error: "Transaction Malformed",
		},
		{
			name: "Test missing operations",
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
								Hint: [4]byte{57, 234, 167, 44},
								Signature: []byte{
									76, 51, 186, 154, 227, 143,
									149, 39, 183, 152, 173, 83,
									50, 63, 221, 130, 197, 118,
									246, 30, 240, 3, 76, 48,
									214, 166, 72, 248, 30, 98,
									172, 161, 138, 67, 145, 48,
									26, 55, 132, 10, 66, 22,
									68, 119, 3, 77, 57, 31,
									236, 107, 181, 221, 226, 227,
									161, 248, 59, 232, 44, 127,
									126, 237, 215, 5,
								},
							},
						},
					},
				},
				Signatures: []xc.TxSignature{
					{
						76, 51, 186, 154, 227, 143,
						149, 39, 183, 152, 173, 83,
						50, 63, 221, 130, 197, 118,
						246, 30, 240, 3, 76, 48,
						214, 166, 72, 248, 30, 98,
						172, 161, 138, 67, 145, 48,
						26, 55, 132, 10, 66, 22,
						68, 119, 3, 77, 57, 31,
						236, 107, 181, 221, 226, 227,
						161, 248, 59, 232, 44, 127,
						126, 237, 215, 5,
					},
				},
			},
			response: "",
			error:    "missing transaction operations",
		},
		{
			name: "Test missing signatures",
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
			response: "",
			error:    "missing transaction signatures",
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(vector.response))
			}))
			defer server.Close()

			txActiveTime, err := time.ParseDuration("2h")
			require.NoError(t, err)
			client, _ := client.NewClient(xc.NewChainConfig(xc.XLM).
				WithUrl(server.URL).
				WithDecimals(7).
				WithTransactionActiveTime(txActiveTime).
				WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(0.00001)).
				WithChainID("Test SDF Network ; September 2015"),
			)

			err = client.SubmitTx(context.Background(), vector.txInput)
			if err != nil {
				require.NotEqual(t, err, "")
				require.ErrorContains(t, err, vector.error)
			} else {
				require.NoError(t, err)
			}
		})
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
	event *xclient.Event,
) *xclient.Movement {
	movement := xclient.NewMovement(xc.NativeAsset(chain), xc.ContractAddress(contract))
	movement.From = from
	movement.To = to
	movement.Event = event
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
					Height: xc.NewAmountBlockchainFromUint64(514624),
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
						nil,
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
						xclient.NewEventFromIndex(0, xclient.MovementVariantFee),
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
				Final:         true,
				State:         xclient.Succeeded,
			},
		},

		{
			// Missing the source account in the operation, like in:
			// https://testnet.stellarchain.io/transactions/bc44695505b047f7dc4a35a99573202220f6c14493eb40ded6b6b3aca0ac65dd
			hash: "bc44695505b047f7dc4a35a99573202220f6c14493eb40ded6b6b3aca0ac65dd",
			getTxResult: `{
				"id": "bc44695505b047f7dc4a35a99573202220f6c14493eb40ded6b6b3aca0ac65dd",
				"paging_token": "5766530465796096",
				"successful": true,
				"hash": "bc44695505b047f7dc4a35a99573202220f6c14493eb40ded6b6b3aca0ac65dd",
				"ledger": 1342625,
				"created_at": "2025-01-09T12:46:09Z",
				"source_account": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
				"source_account_sequence": "338194314821647",
				"fee_account": "GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF",
				"fee_charged": "100",
				"max_fee": "10000",
				"operation_count": 1,
				"envelope_xdr": "AAAAAgAAAAC6dj438wEEaw561ovXxPWFkUhaspPi74/APAysEKvbvwAAJxAACFZwAAAAEAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAQAAAAB7tXKH+4afQYJFcqY/Owzq6xXU4IL05hpMR7ju06lXlAAAAAAAAAAABycOAAAAAAAAAAABEKvbvwAAAEDCITpl3PVMUasdAOAYGxPLDLcK43FMEReLOeEqz33Ja8x05eEJfygqMsuItykcv4dxjgCuyGPmg4OLeIsyR8kO",
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
				"sequence": 1342625,
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
							"paging_token": "5766530465796096",
							"hash": "dbaea72b5a49689c63df5afa0cef9ac40cda1174e9c06612dd8c4d741665c726",
							"prev_hash": "9dffc1f46afdaeae61cbab8f38d4142886b9c26bb5be0127b14643e04e88f46d",
							"sequence": 1342626,
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
				Name:   "chains/XLM/transactions/bc44695505b047f7dc4a35a99573202220f6c14493eb40ded6b6b3aca0ac65dd",
				Hash:   "bc44695505b047f7dc4a35a99573202220f6c14493eb40ded6b6b3aca0ac65dd",
				XChain: xc.NativeAsset("XLM"),
				Block: &xclient.Block{
					Chain:  xc.NativeAsset("XLM"),
					Height: xc.NewAmountBlockchainFromUint64(1342625),
					Hash:   "e7c43d43c778e6e4d3503c59f8226a4f0af36a1ea89b7643c1a77c4296e02d0f",
					Time:   MustParseTime("2025-01-09T12:46:09Z"),
				},
				Movements: []*xclient.Movement{
					NewMovement(
						"XLM",
						"XLM",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(120000000),
								XAddress:  "chains/XLM/addresses/GC5HMPRX6MAQI2YOPLLIXV6E6WCZCSC2WKJ6F34PYA6AZLAQVPN36BG4",
								AddressId: "GC5HMPRX6MAQI2YOPLLIXV6E6WCZCSC2WKJ6F34PYA6AZLAQVPN36BG4",
							},
						},
						[]*xclient.BalanceChange{{
							Balance:   xc.NewAmountBlockchainFromUint64(120000000),
							XAddress:  "chains/XLM/addresses/GB53K4UH7ODJ6QMCIVZKMPZ3BTVOWFOU4CBPJZQ2JRD3R3WTVFLZJ7Q3",
							AddressId: "GB53K4UH7ODJ6QMCIVZKMPZ3BTVOWFOU4CBPJZQ2JRD3R3WTVFLZJ7Q3",
						}},
						nil,
					),
					NewMovement(
						"XLM",
						"XLM",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100),
								XAddress:  "chains/XLM/addresses/GC5HMPRX6MAQI2YOPLLIXV6E6WCZCSC2WKJ6F34PYA6AZLAQVPN36BG4",
								AddressId: "GC5HMPRX6MAQI2YOPLLIXV6E6WCZCSC2WKJ6F34PYA6AZLAQVPN36BG4",
							},
						},
						[]*xclient.BalanceChange{},
						xclient.NewEventFromIndex(0, xclient.MovementVariantFee),
					),
				},
				Fees: []*xclient.Balance{
					{
						Asset:    "chains/XLM/assets/XLM",
						Contract: "XLM",
						Balance:  xc.NewAmountBlockchainFromUint64(100),
					},
				},
				Confirmations: 1,
				Final:         true,
				State:         xclient.Succeeded,
			},
		},
		{
			getTxResult: `{
				"type": "https://stellar.org/horizon-errors/not_found",
				"title": "Resource Missing",
				"status": 404,
				"detail": "The resource at the url requested was not found",
				"extras": {
				}
			}`,
			err: "TransactionNotFound:",
		},
	}

	for _, vector := range vectors {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL.String()
			if vector.err != "" {
				w.WriteHeader(http.StatusBadRequest)
			}

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

		chain := xc.NewChainConfig(xc.XLM).
			WithUrl(server.URL).
			WithTransactionActiveTime(time.Duration(500)).
			WithDecimals(7).
			WithChainID("Test SDF Network ; September 2015").
			WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(0.00001))

		client, err := client.NewClient(chain)
		require.NoError(t, err)
		args := txinfo.NewArgs(xc.TxHash(vector.hash))
		txInfo, err := client.FetchTxInfo(context.Background(), args)
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

func TestFetchBalance(t *testing.T) {
	vectors := []struct {
		assetID          string
		getAccountResult string
		expected         xc.AmountBlockchain
		err              string
	}{
		{
			assetID: "XLM",
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
				},
				{
				  "balance": "5.0",
				  "buying_liabilities": "0.0000000",
				  "selling_liabilities": "0.0000000",
				  "asset_code": "USDC",
				  "asset_issuer": "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"
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
		{
			assetID: "USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5",
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
				},
				{
				  "balance": "5.0",
				  "buying_liabilities": "0.0000000",
				  "selling_liabilities": "0.0000000",
				  "asset_code": "USDC",
				  "asset_issuer": "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"
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
			expected: xc.NewAmountBlockchainFromUint64(50000000),
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

		defaultConfig := xc.NewChainConfig(xc.XLM).
			WithUrl(server.URL).
			WithTransactionActiveTime(time.Duration(500)).
			WithDecimals(7).
			WithChainID("Test SDF Network ; September 2015").
			WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(0.00001))

		cl, err := client.NewClient(defaultConfig)
		require.NoError(t, err)

		args := xclient.NewBalanceArgs(xc.Address("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF"))
		if vector.assetID != "XLM" {
			args.SetContract(xc.ContractAddress(vector.assetID))
		}

		balance, err := cl.FetchBalance(context.Background(), args)
		if vector.err != "" {
			require.ErrorContains(t, err, vector.err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, balance)
			require.Equal(t, vector.expected, balance)
		}
	}
}
