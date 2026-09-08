package client_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/types"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input/txinputtest"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/stretchr/testify/require"
)

func TestFetchLegacyPrevouts(t *testing.T) {
	for _, coin := range []struct {
		native xc.NativeAsset
		p2pk   bool
	}{{xc.BTC, false}, {xc.LTC, false}, {xc.DOGE, false}, {xc.DOGE, true}} {
		for _, provider := range []bitcoin.BitcoinClient{bitcoin.Blockbook, bitcoin.JsonRpc, bitcoin.QuicknodeBlockbook, bitcoin.Blockchair} {
			for _, scenario := range []string{"valid", "understated value", "missing parent", "malformed hex", "wrong parent", "foreign sender"} {
				t.Run(fmt.Sprintf("%s/p2pk=%t/%s/%s", coin.native, coin.p2pk, provider, scenario), func(t *testing.T) {
					script, err := hex.DecodeString("76a914652dac91ff1b130616cb11ce33b0ac2f1b4df89188ac")
					require.NoError(t, err)
					pubkeyHash := script[3:23]
					if coin.p2pk {
						publicKey, err := hex.DecodeString("0205f07274489c05cc2bc603f3f48414442baeabe844ebb340774c3569a45d2646")
						require.NoError(t, err)
						pubkeyHash = btcutil.Hash160(publicKey)
						script = append(append([]byte{33}, publicKey...), 0xac)
					}
					chain := xc.NewChainConfig(coin.native).WithNet("testnet").WithProvider(string(provider))
					network, err := params.GetParams(chain.Base())
					require.NoError(t, err)
					sender, err := btcutil.NewAddressPubKeyHash(pubkeyHash, &network)
					require.NoError(t, err)
					addressScript, err := txscript.PayToAddrScript(sender)
					require.NoError(t, err)
					if scenario == "foreign sender" {
						sender, err = btcutil.NewAddressPubKeyHash(make([]byte, 20), &network)
						require.NoError(t, err)
					}
					dashboardScript, err := txscript.PayToAddrScript(sender)
					require.NoError(t, err)
					from := xc.Address(sender.EncodeAddress())
					outputs, parentRaw := txinputtest.Outputs(t, script, 100_000_000, 20_000_000)
					// A single address can hold both P2PK and P2PKH outputs.
					if coin.p2pk {
						var parent wire.MsgTx
						require.NoError(t, parent.Deserialize(bytes.NewReader(parentRaw)))
						parent.TxOut[2].PkScript = addressScript
						var encoded bytes.Buffer
						require.NoError(t, parent.Serialize(&encoded))
						hash := parent.TxHash()
						for i := range outputs {
							outputs[i].Hash = hash[:]
						}
						parentRaw = encoded.Bytes()
						outputs[1].PubKeyScript = addressScript
					}
					hash, err := chainhash.NewHash(outputs[0].Hash)
					require.NoError(t, err)
					raw := hex.EncodeToString(parentRaw)
					value := 100_000_000
					want := ""
					switch scenario {
					case "foreign sender":
						want = "input script does not match sender"
					case "understated value":
						value = 10_000_000
						want = "value mismatch"
					case "missing parent":
						raw = ""
						want = "missing previous transaction"
					case "malformed hex":
						raw = "zz"
						want = "invalid byte"
					case "wrong parent":
						_, wrongParent := txinputtest.Outputs(t, script, 10_000_000)
						raw = hex.EncodeToString(wrongParent)
						want = "txid does not match"
					}
					utxos := json.RawMessage(fmt.Sprintf(`[{"txid":%q,"vout":1,"value":"%d","height":100},{"txid":%q,"vout":2,"value":"20000000","height":100}]`, hash.String(), value, hash.String()))
					fetches := 0
					txResponse := types.TransactionResponse{Hex: raw, Vout: []types.Vout{{}, {Hex: hex.EncodeToString(outputs[0].PubKeyScript)}, {Hex: hex.EncodeToString(outputs[1].PubKeyScript)}}}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var result interface{}
						if r.Method == http.MethodGet {
							switch {
							case strings.HasPrefix(r.URL.Path, "/dashboards/address/"):
								result = map[string]interface{}{"context": map[string]int{"code": 200}, "data": map[string]interface{}{string(from): map[string]interface{}{
									"address": map[string]string{"script_hex": hex.EncodeToString(dashboardScript)},
									"utxo":    []map[string]interface{}{{"transaction_hash": hash.String(), "index": 1, "value": value, "block_id": 100}, {"transaction_hash": hash.String(), "index": 2, "value": 20_000_000, "block_id": 100}},
								}}}
							case r.URL.Path == "/stats":
								result = map[string]interface{}{"context": map[string]int{"code": 200}, "data": map[string]int{"suggested_transaction_fee_per_byte_sat": 1}}
							case r.URL.Path == "/raw/transaction/"+hash.String():
								fetches++
								result = map[string]interface{}{"context": map[string]int{"code": 200}, "data": map[string]interface{}{hash.String(): map[string]string{"raw_transaction": raw}}}
							case strings.Contains(r.URL.Path, "/utxo/"):
								result = utxos
							case strings.Contains(r.URL.Path, "/estimatefee/"):
								result = map[string]string{"result": "0.00001"}
							case r.URL.Path == "/api/v2/tx/"+hash.String():
								fetches++
								result = txResponse
							default:
								t.Errorf("unexpected path: %s", r.URL.Path)
								w.WriteHeader(404)
								return
							}
						} else {
							var request struct {
								Method string            `json:"method"`
								Params []json.RawMessage `json:"params"`
							}
							if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
								t.Error(err)
								return
							}
							switch request.Method {
							case "bb_getTx":
								result = txResponse
							case "bb_getUTXOs":
								result = utxos
							case "estimatesmartfee":
								result = map[string]interface{}{"feerate": 0.00001, "blocks": 2}
							case "getrawtransaction":
								fetches++
								if len(request.Params) != 2 || string(request.Params[0]) != fmt.Sprintf("%q", hash.String()) || string(request.Params[1]) != "false" {
									t.Errorf("unexpected raw transaction parameters: %s", request.Params)
								}
								result = raw
							default:
								t.Errorf("unexpected method: %s", request.Method)
								w.WriteHeader(400)
								return
							}
							result = map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result}
						}
						if err := json.NewEncoder(w).Encode(result); err != nil {
							t.Error(err)
						}
					}))
					defer server.Close()
					chain = chain.WithUrl(server.URL)
					if provider == bitcoin.Blockchair {
						t.Setenv("TEST_PREVOUT_BLOCKCHAIR_KEY", "test")
						chain = chain.WithAuth("env:TEST_PREVOUT_BLOCKCHAIR_KEY")
					}
					client, err := bitcoin.NewClient(chain)
					require.NoError(t, err)
					args := buildertest.MustNewTransferArgs(chain.ChainBaseConfig, from, from, xc.NewAmountBlockchainFromUint64(1_000_000))
					input, err := client.FetchTransferInput(context.Background(), args)
					// Both client entry points must authenticate legacy values before
					// returning the ordinary, parent-free input format.
					if multiClient, ok := client.(xclient.MultiTransferClient); ok {
						multiArgs, argsErr := xcbuilder.NewMultiTransferArgs(chain.Base(),
							[]*xcbuilder.Sender{buildertest.MustNewSender(from, nil)},
							[]*xcbuilder.Receiver{buildertest.MustNewReceiver(from, xc.NewAmountBlockchainFromUint64(1_000_000))})
						require.NoError(t, argsErr)
						before := fetches
						multiInput, multiErr := multiClient.FetchMultiTransferInput(context.Background(), *multiArgs)
						if want != "" {
							require.ErrorContains(t, multiErr, want)
						} else {
							require.NoError(t, multiErr)
							require.Equal(t, input.(*tx_input.TxInput).UnspentOutputs, multiInput.(*tx_input.MultiTransferInput).Inputs[0].UnspentOutputs)
							encoded, encodeErr := json.Marshal(multiInput)
							require.NoError(t, encodeErr)
							require.NotContains(t, string(encoded), "previous_tx")
							require.Equal(t, before, fetches-before, "each client call should fetch a shared parent once")
						}
						fetches = before
					}
					if want != "" {
						require.ErrorContains(t, err, want)
						return
					}
					require.NoError(t, err)
					expectedFetches := 1
					// Dogecoin's existing discovery path also fetches each output's script.
					if coin.native == xc.DOGE && (provider == bitcoin.Blockbook || provider == bitcoin.JsonRpc) {
						expectedFetches += 2
					}
					require.Equal(t, expectedFetches, fetches, "shared parent should only be fetched once for verification")
					encodedInput, err := json.Marshal(input)
					require.NoError(t, err)
					require.NotContains(t, string(encodedInput), "previous_tx")
					selected := input.(*tx_input.TxInput).UnspentOutputs
					require.Len(t, selected, 2)
					for _, output := range selected {
						require.Equal(t, outputs[output.Index-1].PubKeyScript, output.PubKeyScript)
					}
				})
			}
		}
	}
}
