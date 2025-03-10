package client_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/filecoin/address"
	client "github.com/cordialsys/crosschain/chain/filecoin/client"
	"github.com/cordialsys/crosschain/chain/filecoin/client/types"
	"github.com/cordialsys/crosschain/chain/filecoin/tx"
	"github.com/cordialsys/crosschain/chain/filecoin/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client, err := client.NewClient(xc.NewChainConfig(""))
	require.NotNil(t, client)
	require.NoError(t, err)
}

func TestFetchTxInput(t *testing.T) {
	vectors := []struct {
		name                  string
		provide_to            bool
		amount                xc.AmountBlockchain
		getChainHeadResult    string
		getNonceResult        string
		estimateGasFeesResult string
		err                   string
		expectedInput         *tx_input.TxInput
		maxGasLimit           uint64
		maxGasFeeCap          float64
	}{
		{
			name:   "ValidInput",
			amount: xc.NewAmountBlockchainFromStr("1"),
			getChainHeadResult: `{"result":{
              "Cids": [
                {
                  "/": "bafy2bzacea7kciha7midmmnermmx5dtaj3gjo2lghjkozd5t6diclbwngvnka"
                },
                {
                  "/": "bafy2bzaceaflojujuuu5gzh4jmlnf4dgtrgyxugv4eechwwdh4huwre3mvj2q"
                }
              ],
              "Height": 2441564
            }}`,
			getNonceResult: `{"result": 21}`,
			estimateGasFeesResult: `{"result": {
			    "CID":{"/":"bafy2bzacedlpdofjj2aturtq42bk4j6qgmrvcs7lkmtn6x7goqgf3n4pvqx7o"},
			    "From":"f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			    "GasFeeCap":"100542",
			    "GasLimit":1518203,
			    "GasPremium":"99488",
			    "Method":0,
			    "Nonce":21,
			    "Params":null,
			    "To":"f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
			    "Value":"1",
			    "Version":42
			  }
			}`,
			expectedInput: &tx_input.TxInput{
				Nonce:      21,
				GasLimit:   1518203,
				GasFeeCap:  xc.NewAmountBlockchainFromStr("100542"),
				GasPremium: xc.NewAmountBlockchainFromStr("99488"),
			},
		},
		{
			name:       "MissingToAddress",
			amount:     xc.NewAmountBlockchainFromStr("1"),
			provide_to: false,
			getChainHeadResult: `{"result":{
              "Cids": [
                {
                  "/": "bafy2bzacea7kciha7midmmnermmx5dtaj3gjo2lghjkozd5t6diclbwngvnka"
                },
                {
                  "/": "bafy2bzaceaflojujuuu5gzh4jmlnf4dgtrgyxugv4eechwwdh4huwre3mvj2q"
                }
              ],
              "Height": 2441564
            }}`,
			getNonceResult: `{"result": 21}`,
			estimateGasFeesResult: `{"result": {
			    "CID":{"/":"bafy2bzacedlpdofjj2aturtq42bk4j6qgmrvcs7lkmtn6x7goqgf3n4pvqx7o"},
			    "From":"f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			    "GasFeeCap":"100542",
			    "GasLimit":1518203,
			    "GasPremium":"99488",
			    "Method":0,
			    "Nonce":21,
			    "Params":null,
			    "To":"f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			    "Value":"1",
			    "Version":42
			  }
			}`,
			expectedInput: &tx_input.TxInput{
				Nonce:      21,
				GasLimit:   1518203,
				GasFeeCap:  xc.NewAmountBlockchainFromStr("100542"),
				GasPremium: xc.NewAmountBlockchainFromStr("99488"),
			},
		},
		{
			name:           "MissingGetChainHead",
			amount:         xc.NewAmountBlockchainFromStr("1"),
			getNonceResult: `{"result": 21}`,
			estimateGasFeesResult: `{"result": {
			    "CID":{"/":"bafy2bzacedlpdofjj2aturtq42bk4j6qgmrvcs7lkmtn6x7goqgf3n4pvqx7o"},
			    "From":"f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			    "GasFeeCap":"100542",
			    "GasLimit":1518203,
			    "GasPremium":"99488",
			    "Method":0,
			    "Nonce":21,
			    "Params":null,
			    "To":"f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
			    "Value":"1",
			    "Version":42
			  }
			}`,
			err: "invalid input",
		},
		{
			name:   "MissingGetNonce",
			amount: xc.NewAmountBlockchainFromStr("1"),
			getChainHeadResult: `{"result":{
              "Cids": [
                {
                  "/": "bafy2bzacea7kciha7midmmnermmx5dtaj3gjo2lghjkozd5t6diclbwngvnka"
                },
                {
                  "/": "bafy2bzaceaflojujuuu5gzh4jmlnf4dgtrgyxugv4eechwwdh4huwre3mvj2q"
                }
              ],
              "Height": 2441564
            }}`,
			estimateGasFeesResult: `{"result": {
			    "CID":{"/":"bafy2bzacedlpdofjj2aturtq42bk4j6qgmrvcs7lkmtn6x7goqgf3n4pvqx7o"},
			    "From":"f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			    "GasFeeCap":"100542",
			    "GasLimit":1518203,
			    "GasPremium":"99488",
			    "Method":0,
			    "Nonce":21,
			    "Params":null,
			    "To":"f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
			    "Value":"1",
			    "Version":42
			  }
			}`,
			err: "invalid input",
		},
		{
			name:   "MissingEstimateGasFees",
			amount: xc.NewAmountBlockchainFromStr("1"),
			getChainHeadResult: `{"result":{
              "Cids": [
                {
                  "/": "bafy2bzacea7kciha7midmmnermmx5dtaj3gjo2lghjkozd5t6diclbwngvnka"
                },
                {
                  "/": "bafy2bzaceaflojujuuu5gzh4jmlnf4dgtrgyxugv4eechwwdh4huwre3mvj2q"
                }
              ],
              "Height": 2441564
            }}`,
			getNonceResult: `{"result": 21}`,
			err:            "invalid input",
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqBuff, _ := io.ReadAll(r.Body)
				rawReq := string(reqBuff)
				if strings.Contains(rawReq, types.MethodChainHead) {
					w.Write([]byte(v.getChainHeadResult))
				} else if strings.Contains(rawReq, types.MethodMpoolGetNonce) {
					w.Write([]byte(v.getNonceResult))
				} else {
					w.Write([]byte(v.estimateGasFeesResult))
				}
			}))
			defer server.Close()

			client, _ := client.NewClient(
				xc.NewChainConfig(xc.FIL, xc.DriverFilecoin).
					WithUrl(server.URL).
					WithNet("mainnet").
					WithGasPriceMultiplier(v.maxGasFeeCap).
					WithMaxGasPrice(v.maxGasFeeCap).
					WithDecimals(18),
			)
			from := xc.Address("f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy")
			to := xc.Address("")
			if v.provide_to {
				to = xc.Address("f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q")
			}
			input, err := client.FetchLegacyTxInput(context.Background(), from, to)
			if v.err != "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)

				input.(*tx_input.TxInput).XGasFeeCap = xc.AmountBlockchain{}
				input.(*tx_input.TxInput).XGasPremium = xc.AmountBlockchain{}
				input.(*tx_input.TxInput).XGasLimit = 0
				input.(*tx_input.TxInput).XNonce = 0
				require.Equal(t, v.expectedInput, input)
			}
		})
	}
}

func TestSubmitTx(t *testing.T) {
	vec := []struct {
		name     string
		tx       *tx.Tx
		response string
		err      string
	}{
		{
			name: "ValidTx",
			tx: &tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
				Signature: tx.Signature{
					Type: address.ProtocolSecp256k1,
					Data: []byte("btOGs/+MfKwi02EQIdhvPdj8fw6xizsfCN6nMWCaR9YTSm8+ZjqYP5ggE8GzW0UrJd1zDgc1FNEwJYT6cxWElgE=\\"),
				},
			},
			response: `{"/":"bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"}`,
		},
		{
			name: "InvalidSignature",
			tx: &tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
				Signature: tx.Signature{
					Type: address.ProtocolSecp256k1,
					Data: []byte("totally invalid signature"),
				},
			},
			response: `{"code":1,"message":"signature verification failed: failed to validate signature: invalid signature for message bafy2bzacecki4bg53vhveytdjr4l4opy7fbmlgpujorvlxyfo2sz45fmwnafi (type 1): signature did not match"}`,
			err:      "invalid signature for message bafy2bzacecki4bg53vhveytdjr4l4opy7fbmlgpujorvlxyfo2sz45fmwnafi",
		},
	}
	for _, v := range vec {
		t.Run(v.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(v.response))
			}))
			defer server.Close()

			client, _ := client.NewClient(xc.NewChainConfig(xc.FIL, xc.DriverFilecoin).WithUrl(server.URL).WithNet("mainnet"))
			err := client.SubmitTx(context.Background(), v.tx)
			if err != nil {
				require.Error(t, err)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
			}
		})

	}
}

func TestFetchTxInfo(t *testing.T) {
	vectors := []struct {
		name                    string
		hash                    string
		chainGetMessageResponse string
		stateSearchMsgResponse  string
		chainGetBlockResponse   string
		chainGetHeadResponse    string
		expectedInfo            xclient.TxInfo
		err                     string
	}{
		{
			name: "ValidTx",
			hash: "bafy2bzacedza344ak7eol4uydlwddlj6igiseftbaomafc3iscsmzoslo65vc",
			chainGetMessageResponse: `{"result":{
			  "CID":{"/":"bafy2bzacea5vwrckqz2lin5snjrvuihwdfwumu6hqaibnxwl7dkjwfrntchhi"},
			  "From":"f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			  "GasFeeCap":"101183",
			  "GasLimit":1518203,
			  "GasPremium":"100129",
			  "Method":0,
			  "Nonce":16,
			  "Params":null,
			  "To":"f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
			  "Value":"100000000000000000",
			  "Version":0
			}}`,
			stateSearchMsgResponse: `{"result":{
			  "Height":2440037,
			  "Message":{"/":"bafy2bzacedza344ak7eol4uydlwddlj6igiseftbaomafc3iscsmzoslo65vc"},
			  "Receipt":{
			    "EventsRoot":null,
			    "ExitCode":0,
			    "GasUsed":1227563,
			    "Return":null
			  },
			  "TipSet":[
			    {"/":"bafy2bzacectazjjqvph7rf552wthajzstqy7ylugrstmrloq2m7a6gpri3py6"},
			    {"/":"bafy2bzacebg3wk2cr62qcfovmrslxtgm23h5v3la7bztimm2iltyke3hgebkg"}
			  ]
			}}`,
			chainGetBlockResponse: `{"result":{
			  "ParentBaseFee":"100",
			  "Timestamp":1740527490,
			  "Height":2440037
			}}`,
			chainGetHeadResponse: `{"result":{
			  "Cids":[
			    {"/":"bafy2bzaceaoat7pamoo65dedvdyr63ilpilma6q6u3zi4aktkrhndrg7ehxju"},
			    {"/":"bafy2bzacebgtznsyyctrlhivrscqoqqj6z23fwmtlwdsnqtenbtn2ia2muek6"},
			    {"/":"bafy2bzacecviu3uzrpvkxcpcekskffs4d6turxs5p3ehyb2gfbynmxomzi7ku"}
			  ],
			  "Height":2441667
			}}`,
			expectedInfo: xclient.TxInfo{
				Name:   xclient.TransactionName("chains/FIL/transactions/bafy2bzacedza344ak7eol4uydlwddlj6igiseftbaomafc3iscsmzoslo65vc"),
				Hash:   "bafy2bzacedza344ak7eol4uydlwddlj6igiseftbaomafc3iscsmzoslo65vc",
				XChain: xc.NativeAsset("FIL"),
				Final:  true,
				Block: &xclient.Block{
					Chain:  xc.NativeAsset("FIL"),
					Height: xc.NewAmountBlockchainFromUint64(2440037),
					Hash:   "bafy2bzacedza344ak7eol4uydlwddlj6igiseftbaomafc3iscsmzoslo65vc",
					Time:   MustParseTime("1970-01-21T03:28:47.49Z"),
				},
				Movements: []*xclient.Movement{
					NewMovement(
						"FIL",
						"FIL",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100000000000000000),
								XAddress:  "chains/FIL/addresses/f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
								AddressId: "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
							},
						},
						[]*xclient.BalanceChange{{
							Balance:   xc.NewAmountBlockchainFromUint64(100000000000000000),
							XAddress:  "chains/FIL/addresses/f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
							AddressId: "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
						}},
					),
					NewMovement(
						"FIL",
						"FIL",
						[]*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(152138904487),
								XAddress:  "chains/FIL/addresses/f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
								AddressId: "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
							},
						},
						[]*xclient.BalanceChange{},
					),
				},
				Fees: []*xclient.Balance{
					{
						Asset:    "chains/FIL/assets/FIL",
						Contract: "FIL",
						Balance:  xc.NewAmountBlockchainFromUint64(152138904487),
					},
				},
				Confirmations: 1630,
			},
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqBuff, _ := io.ReadAll(r.Body)
				rawReq := string(reqBuff)
				if strings.Contains(rawReq, types.MethodChainGetMessage) {
					w.Write([]byte(v.chainGetMessageResponse))
				} else if strings.Contains(rawReq, types.MethodStateSearchMsg) {
					w.Write([]byte(v.stateSearchMsgResponse))
				} else if strings.Contains(rawReq, types.MethodChainGetBlock) {
					w.Write([]byte(v.chainGetBlockResponse))
				} else {
					w.Write([]byte(v.chainGetHeadResponse))
				}
			}))
			defer server.Close()

			client, _ := client.NewClient(
				xc.NewChainConfig(xc.FIL, xc.DriverFilecoin).
					WithUrl(server.URL).
					WithNet("mainnet").
					WithDecimals(18),
			)

			txInfo, err := client.FetchTxInfo(context.Background(), xc.TxHash(v.hash))
			if v.err != "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, txInfo)
				require.Equal(t, v.expectedInfo, txInfo)
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
) *xclient.Movement {
	movement := xclient.NewMovement(xc.NativeAsset(chain), xc.ContractAddress(contract))
	movement.From = from
	movement.To = to

	return movement
}
