package client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	solanabuilder "github.com/cordialsys/crosschain/chain/solana/builder"
	"github.com/cordialsys/crosschain/chain/solana/client"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/solana-foundation/solana-go/v2"
	"github.com/stretchr/testify/require"
)

func TestV1TransferClient(t *testing.T) {
	for _, simulationFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "simulation failure"}[simulationFails], func(t *testing.T) {
			sender := solana.NewWallet()
			recipient := solana.NewWallet().PublicKey()
			var signed *solana.Transaction
			var wire []byte
			var simulated, submitted, fetched bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
				var result any
				switch request.Method {
				case "getLatestBlockhash":
					result = map[string]any{"context": map[string]any{"slot": 12}, "value": map[string]any{"blockhash": solana.Hash{3}.String(), "lastValidBlockHeight": 100}}
				case "getMinimumBalanceForRentExemption":
					result = 1_447_680
				case "getBalance":
					result = map[string]any{"context": map[string]any{"slot": 12}, "value": 0}
				case "getAccountInfo":
					result = map[string]any{"value": nil}
				case "simulateTransaction":
					simulated = true
					var encoded string
					require.NoError(t, json.Unmarshal(request.Params[0], &encoded))
					binary, err := base64.StdEncoding.DecodeString(encoded)
					require.NoError(t, err)
					require.Equal(t, byte(0x81), binary[0])
					transaction, err := solana.TransactionFromBytes(binary)
					require.NoError(t, err)
					require.Equal(t, tx_input.MaxComputeUnitLimit, *transaction.Message.TransactionConfig.ComputeUnitLimit)
					require.Equal(t, tx_input.MaxLoadedAccountsDataSizeLimit, *transaction.Message.TransactionConfig.LoadedAccountsDataSizeLimit)
					var simulationError any
					if simulationFails {
						simulationError = "AccountNotFound"
					}
					result = map[string]any{"value": map[string]any{"err": simulationError, "unitsConsumed": 20_001, "loadedAccountsDataSize": 32_769}}
				case "sendTransaction":
					submitted = true
					var encoded string
					require.NoError(t, json.Unmarshal(request.Params[0], &encoded))
					require.Equal(t, base64.StdEncoding.EncodeToString(wire), encoded)
					result = signed.Signatures[0].String()
				case "getTransaction":
					fetched = true
					var opts map[string]any
					require.NoError(t, json.Unmarshal(request.Params[1], &opts))
					require.Equal(t, float64(1), opts["maxSupportedTransactionVersion"])
					require.Equal(t, "base64", opts["encoding"])
					result = map[string]any{"slot": 10, "blockTime": 123456, "version": 1,
						"transaction": []string{base64.StdEncoding.EncodeToString(wire), "base64"},
						"meta":        map[string]any{"err": nil, "fee": 5000, "preBalances": []uint64{100000, 0, 1}, "postBalances": []uint64{85000, 10000, 1}}}
				default:
					t.Errorf("unexpected RPC: %s", request.Method)
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}))
			}))
			defer server.Close()
			cfg := xc.NewChainConfig(xc.SOL)
			cfg.URL = server.URL
			c, err := client.NewClient(cfg)
			require.NoError(t, err)
			args, err := xcbuilder.NewTransferArgs(cfg.Base(), xc.Address(sender.PublicKey().String()), xc.Address(recipient.String()), xc.NewAmountBlockchainFromUint64(10000), xcbuilder.OptionTransactionVersion("v1"))
			require.NoError(t, err)
			input, err := c.FetchTransferInput(context.Background(), args)
			if simulationFails {
				require.ErrorContains(t, err, "simulation failed")
				require.False(t, submitted)
				return
			}
			require.NoError(t, err)
			require.True(t, simulated)
			require.Equal(t, uint32(24002), input.(*tx_input.TxInput).ComputeUnitLimit)
			require.Equal(t, uint32(65536), input.(*tx_input.TxInput).LoadedAccountsDataSizeLimit)
			b, err := solanabuilder.NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			built, err := b.Transfer(args, input)
			require.NoError(t, err)
			requests, err := built.Sighashes()
			require.NoError(t, err)
			sig, err := sender.PrivateKey.Sign(requests[0].Payload)
			require.NoError(t, err)
			require.NoError(t, built.SetSignatures(&xc.SignatureResponse{Signature: sig[:]}))
			signed = built.(*tx.Tx).SolTx
			wire, err = built.Serialize()
			require.NoError(t, err)
			require.NoError(t, signed.VerifySignatures())
			req, err := xctypes.SubmitTxReqFromTx(xc.SOL, built)
			require.NoError(t, err)
			require.NoError(t, c.SubmitTx(context.Background(), req, xcbuilder.SubmitArgs{}))
			info, err := c.FetchLegacyTxInfo(context.Background(), built.Hash())
			require.NoError(t, err)
			require.True(t, submitted && fetched)
			require.Equal(t, args.GetFrom(), info.From)
			require.Equal(t, args.GetTo(), info.To)
			require.Equal(t, uint64(10000), info.Amount.Uint64())
			require.Equal(t, uint64(5000), info.Fee.Uint64())
		})
	}
}
