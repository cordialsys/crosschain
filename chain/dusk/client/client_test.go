package client_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/dusk/client"
	tx "github.com/cordialsys/crosschain/chain/dusk/tx"
	"github.com/cordialsys/crosschain/chain/dusk/tx_input"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	url := "http://localhost:8080"
	client, err := client.NewClient(&xc.ChainConfig{
		ChainBaseConfig: &xc.ChainBaseConfig{},
		ChainClientConfig: &xc.ChainClientConfig{
			URL: url,
		},
	})

	require.NotNil(t, client)
	require.NoError(t, err)
	require.Equal(t, client.RuesUrl, fmt.Sprintf("%s/on", url))
}

func TestFetchTxInput(t *testing.T) {
	vectors := []struct {
		testName            string
		accountStatusResult string
		chainIdResult       string
		feeLimit            xc.AmountHumanReadable
		gasFeePriority      *xc.GasFeePriority
		expected            tx_input.TxInput
	}{
		{
			testName:            "TestInputNoGasPriority",
			accountStatusResult: `{"balance":100,"nonce":20}`,
			chainIdResult:       "\x01",
			feeLimit:            xc.NewAmountHumanReadableFromFloat(2.0),
			gasFeePriority:      nil,
			expected: tx_input.TxInput{
				// Incremented nonce by 1
				Nonce: 21,
				// Estimated basing on feeLimit
				GasLimit: 2_000_000_000, // feeLimit / price
				// Always 1 for now
				GasPrice: 1,
				ChainId:  1,
			},
		},
		{
			testName:            "LowGasPriority",
			accountStatusResult: `{"balance":100,"nonce":20}`,
			chainIdResult:       "\x01",
			feeLimit:            xc.NewAmountHumanReadableFromFloat(2.0),
			gasFeePriority:      &xc.Low,
			expected: tx_input.TxInput{
				// Incremented nonce by 1
				Nonce: 21,
				// Estimated basing on feeLimit
				GasLimit: 1400000000, // (feeLimit / price) * 0.7
				// Always 1 for now
				GasPrice: 1,
				ChainId:  1,
			},
		},
		{
			testName:            "CustomGasPriority",
			accountStatusResult: `{"balance":100,"nonce":20}`,
			chainIdResult:       "\x01",
			feeLimit:            xc.NewAmountHumanReadableFromFloat(2.0),
			gasFeePriority:      func() *xc.GasFeePriority { p, _ := xc.NewPriority("0.05"); return &p }(),
			expected: tx_input.TxInput{
				// Incremented nonce by 1
				Nonce: 21,
				// Estimated basing on feeLimit
				GasLimit: 50_000_000, // (feeLimit / price) * 0.7
				// Always 1 for now
				GasPrice: 1,
				ChainId:  1,
			},
		},
	}

	for _, vector := range vectors {
		t.Run(vector.testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := r.URL
				if strings.Contains(url.Path, "account") {
					_, err := w.Write([]byte(vector.accountStatusResult))
					require.NoError(t, err)
				} else {
					_, err := w.Write([]byte(vector.chainIdResult))
					require.NoError(t, err)
				}
			}))
			defer server.Close()

			config := xc.NewChainConfig(xc.DUSK).
				WithUrl(server.URL).
				WithFeeLimit(vector.feeLimit).
				WithDecimals(9)

			client, err := client.NewClient(config)
			require.NoError(t, err)

			input, err := client.FetchLegacyTxInput(context.Background(), xc.Address("from"), xc.Address("to"))
			require.NoError(t, err)
			if vector.gasFeePriority != nil {
				err := input.SetGasFeePriority(*vector.gasFeePriority)
				require.NoError(t, err)
			}
			dusk_input := input.(*tx_input.TxInput)
			require.Equal(t, vector.expected.Nonce, dusk_input.Nonce)
			require.Equal(t, vector.expected.GasLimit, dusk_input.GasLimit)
			require.Equal(t, vector.expected.GasPrice, dusk_input.GasPrice)
			require.Equal(t, vector.expected.ChainId, dusk_input.ChainId)
		})
	}
}

func TestSubmitTx(t *testing.T) {
	config := xc.NewChainConfig(xc.DUSK).
		WithFeeLimit(xc.NewAmountHumanReadableFromFloat(2.0)).
		WithDecimals(9)
	from := xc.Address("2293LeWtYGpsBA99HRg2AfMm9oYhikZ83GSW5NP6QtQxDvkBTAdU8LfQj9fXvDt1rK1baqBcf3gQKsLXpw3LUjpdkSMRMrTsfuTo5Yri1xvUDnVcMMpgTG4o7ThCjZuLMp9L")
	to := xc.Address("26nbWp93it1FF8ChyBUmV2zrXMqsv6xR41UUfcyq37abhoYvvEW4C8MgJPdKnzfQhfa6t1VtVj2QUeDK1aP98TGGtumV897Gtv3M7mh2qZBNK6C4LqvP6GyTeHvC7kPncVvg")
	args, err := builder.NewTransferArgs(
		config.ChainBaseConfig,
		from,
		to,
		xc.NewAmountBlockchainFromUint64(5_000_000),
	)
	require.NoError(t, err)
	exampleTx, err := tx.NewTx(args, tx_input.TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{},
		Nonce:           10,
		GasLimit:        2_500_000,
		GasPrice:        1,
		ChainId:         1,
	})
	require.NoError(t, err)
	exampleTx.Signature = []byte{138, 52, 141, 88, 247, 205, 110, 26, 136, 4, 115, 92, 9, 180, 157, 74, 111, 167, 81, 176, 40, 192, 82, 165, 224, 187, 10, 48, 123, 54, 6, 103, 91, 40, 171, 11, 228, 111, 194, 56, 33, 140, 131, 4, 134, 17, 126, 228}

	vectors := []struct {
		testName string
		tx       *tx.Tx
		err      string
	}{
		{
			testName: "ValidTx",
			tx:       &exampleTx,
		},
		{
			testName: "ValidTx",
			tx:       &exampleTx,
			err:      "this transaction exists in the mempool",
		},
	}

	for _, vector := range vectors {
		t.Run(vector.testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if vector.err != "" {
					w.WriteHeader(http.StatusInternalServerError)
					_, err := w.Write([]byte(vector.err))
					require.NoError(t, err)
				} else {
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte(""))
					require.NoError(t, err)
				}
			}))
			defer server.Close()

			config = config.WithUrl(server.URL)

			client, err := client.NewClient(config)
			require.NoError(t, err)

			tx, err := xctypes.SubmitTxReqFromTx(vector.tx)
			require.NoError(t, err)
			err = client.SubmitTx(context.Background(), tx)
			if vector.err != "" {
				require.ErrorContains(t, err, vector.err)
				return
			}
		})
	}
}

func TestFetchTxInfo(t *testing.T) {
	vectors := []struct {
		testName             string
		lastBlockResult      string
		getTransactionResult string
		expectedInfo         txinfo.TxInfo
		err                  string
	}{
		{
			testName:             "ValidTx",
			lastBlockResult:      `{"lastBlockPair":{"json":{"last_block":[732552,"53a33b68a16a2d109ef6709c9f9a555b68005114d9ca5d2107df359c8cd70e43"],"last_finalized_block":[732551,"b929b84337f3c42552df162d015f2d93a4c5d6c8d439bb789e90a7c6525db139"]}}}`,
			getTransactionResult: `{"tx":{"blockHash":"2c85f57c99b3b038b204a7119acb6356d97fc8c5dab0a5738bae0edb493867d4","blockHeight":723067,"blockTimestamp":1743489829,"err":null,"gasSpent":103396,"id":"98f64bce83ff51a179ca04ed5fa41d63b58b5d4044f575f12457d7151da4401d","tx":{"json":"{\"type\":\"moonlight\",\"sender\":\"zFVXTQZfY6gB2rWZFEm4ToWY2NKrpgeswTboWMAe7cjzaqHjsdGuADp8GsYzeruRkP3ZozA34mhWedEHkEiUr73iv8EcK3A192Y5b2uQYxLgprkZ2u8WKZyfDuZiyy4yEuL\",\"receiver\":\"23jFPiYSVEabmdo3zENKt4qSdBsVMDRTcneZ7LYKhfuB7E8UHXCLiPKDwXWTx4DAQirk2PhVkh6PiZKzaYhAkHuRwsKc7EAaB44yETLLzaWPoBGYDeoiwE82x9oY913RFUPH\",\"value\":1450000000000,\"nonce\":41,\"deposit\":0,\"fee\":{\"gas_limit\":\"100000000\",\"gas_price\":\"1\",\"refund_address\":\"zFVXTQZfY6gB2rWZFEm4ToWY2NKrpgeswTboWMAe7cjzaqHjsdGuADp8GsYzeruRkP3ZozA34mhWedEHkEiUr73iv8EcK3A192Y5b2uQYxLgprkZ2u8WKZyfDuZiyy4yEuL\"},\"call\":null,\"is_deploy\":false,\"memo\":null}"}}}`,
			expectedInfo: txinfo.TxInfo{
				Name:   txinfo.TransactionName("chains/DUSK/transactions/98f64bce83ff51a179ca04ed5fa41d63b58b5d4044f575f12457d7151da4401d"),
				Hash:   "98f64bce83ff51a179ca04ed5fa41d63b58b5d4044f575f12457d7151da4401d",
				XChain: xc.NativeAsset("DUSK"),
				State:  txinfo.Succeeded,
				Final:  true,
				Block: &txinfo.Block{
					Chain:  xc.NativeAsset("DUSK"),
					Height: xc.NewAmountBlockchainFromUint64(723067),
					Hash:   "2c85f57c99b3b038b204a7119acb6356d97fc8c5dab0a5738bae0edb493867d4",
					Time:   MustParseTime("2025-04-01T08:43:49+02:00"),
				},
				Movements: []*txinfo.Movement{
					NewMovement(
						"DUSK",
						"DUSK",
						[]*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(1450000000000),
								XAddress:  "chains/DUSK/addresses/zFVXTQZfY6gB2rWZFEm4ToWY2NKrpgeswTboWMAe7cjzaqHjsdGuADp8GsYzeruRkP3ZozA34mhWedEHkEiUr73iv8EcK3A192Y5b2uQYxLgprkZ2u8WKZyfDuZiyy4yEuL",
								AddressId: "zFVXTQZfY6gB2rWZFEm4ToWY2NKrpgeswTboWMAe7cjzaqHjsdGuADp8GsYzeruRkP3ZozA34mhWedEHkEiUr73iv8EcK3A192Y5b2uQYxLgprkZ2u8WKZyfDuZiyy4yEuL",
							},
						},
						[]*txinfo.BalanceChange{{
							Balance:   xc.NewAmountBlockchainFromUint64(1450000000000),
							XAddress:  "chains/DUSK/addresses/23jFPiYSVEabmdo3zENKt4qSdBsVMDRTcneZ7LYKhfuB7E8UHXCLiPKDwXWTx4DAQirk2PhVkh6PiZKzaYhAkHuRwsKc7EAaB44yETLLzaWPoBGYDeoiwE82x9oY913RFUPH",
							AddressId: "23jFPiYSVEabmdo3zENKt4qSdBsVMDRTcneZ7LYKhfuB7E8UHXCLiPKDwXWTx4DAQirk2PhVkh6PiZKzaYhAkHuRwsKc7EAaB44yETLLzaWPoBGYDeoiwE82x9oY913RFUPH",
						}},
						nil,
					),
					NewMovement(
						"DUSK",
						"DUSK",
						[]*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(103396),
								XAddress:  "chains/DUSK/addresses/zFVXTQZfY6gB2rWZFEm4ToWY2NKrpgeswTboWMAe7cjzaqHjsdGuADp8GsYzeruRkP3ZozA34mhWedEHkEiUr73iv8EcK3A192Y5b2uQYxLgprkZ2u8WKZyfDuZiyy4yEuL",
								AddressId: "zFVXTQZfY6gB2rWZFEm4ToWY2NKrpgeswTboWMAe7cjzaqHjsdGuADp8GsYzeruRkP3ZozA34mhWedEHkEiUr73iv8EcK3A192Y5b2uQYxLgprkZ2u8WKZyfDuZiyy4yEuL",
							},
						},
						[]*txinfo.BalanceChange{},
						txinfo.NewEventFromIndex(0, txinfo.MovementVariantFee),
					),
				},
				Fees: []*txinfo.Balance{
					{
						Asset:    "chains/DUSK/assets/DUSK",
						Contract: "DUSK",
						Balance:  xc.NewAmountBlockchainFromUint64(103396),
					},
				},
				Confirmations: 9484,
				Error:         nil,
			},
		},
	}

	for _, vector := range vectors {
		t.Run(vector.testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqBuff, _ := io.ReadAll(r.Body)
				rawReq := string(reqBuff)
				if strings.Contains(rawReq, "lastBlockPair") {
					_, err := w.Write([]byte(vector.lastBlockResult))
					require.NoError(t, err)
				} else {
					_, err := w.Write([]byte(vector.getTransactionResult))
					require.NoError(t, err)
				}
			}))
			defer server.Close()

			config := xc.NewChainConfig(xc.DUSK, xc.DriverDusk).
				WithUrl(server.URL).
				WithFeeLimit(xc.NewAmountHumanReadableFromFloat(2.0)).
				WithDecimals(9)

			client, err := client.NewClient(config)
			require.NoError(t, err)

			args := txinfo.NewArgs(xc.TxHash(vector.expectedInfo.Hash))
			txInfo, err := client.FetchTxInfo(context.Background(), args)
			require.NoError(t, err)
			require.Equal(t, vector.expectedInfo, txInfo)
		})
	}
}

func NewMovement(
	chain string,
	contract string,
	from []*txinfo.BalanceChange,
	to []*txinfo.BalanceChange,
	event *txinfo.Event,
) *txinfo.Movement {
	movement := txinfo.NewMovement(xc.NativeAsset(chain), xc.ContractAddress(contract))
	movement.From = from
	movement.To = to
	movement.Event = event
	return movement
}

func MustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("failed to parse time")
	}

	unixTime := t.Unix()
	return time.Unix(unixTime, 0)
}
