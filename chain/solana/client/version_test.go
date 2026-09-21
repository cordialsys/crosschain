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
	solanacall "github.com/cordialsys/crosschain/chain/solana/call"
	"github.com/cordialsys/crosschain/chain/solana/client"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/factory"
	"github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferSimulationVersionFallback(t *testing.T) {
	const decodeError = "failed to deserialize solana_sdk::transaction::versioned::VersionedTransaction: io error: failed to fill whole buffer"
	for _, tc := range []struct {
		name           string
		message        string
		explicitConfig bool
		failV0         bool
		wantCalls      int
		wantError      bool
	}{
		{name: "older validator", message: decodeError, wantCalls: 2},
		{name: "unrelated invalid params", message: "Invalid param: invalid encoding", wantCalls: 1, wantError: true},
		{name: "explicit v1 config", message: decodeError, explicitConfig: true, wantCalls: 1, wantError: true},
		{name: "v0 also rejected", message: decodeError, failV0: true, wantCalls: 2, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
					return
				}
				assert.Equal(t, "simulateTransaction", request.Method)
				if !assert.NotEmpty(t, request.Params) {
					return
				}
				var encoded string
				if !assert.NoError(t, json.Unmarshal(request.Params[0], &encoded)) {
					return
				}
				wire, err := base64.StdEncoding.DecodeString(encoded)
				if !assert.NoError(t, err) {
					return
				}
				transaction, err := solana.TransactionFromBytes(wire)
				if !assert.NoError(t, err) {
					return
				}
				calls++
				version := solana.MessageVersionV1
				if calls > 1 {
					version = solana.MessageVersionV0
				}
				assert.Equal(t, version, transaction.Message.GetVersion())
				response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
				if calls == 1 || tc.failV0 {
					response["error"] = map[string]any{"code": -32602, "message": tc.message}
				} else {
					response["result"] = map[string]any{"value": map[string]any{"err": nil, "unitsConsumed": 20001}}
				}
				assert.NoError(t, json.NewEncoder(w).Encode(response))
			}))
			defer server.Close()
			cfg := xc.NewChainConfig(xc.SOL)
			c := &client.Client{SolClient: rpc.New(server.URL), Asset: cfg}
			args, err := xcbuilder.NewTransferArgs(cfg.Base(), xc.Address(solana.NewWallet().PublicKey().String()), xc.Address(solana.NewWallet().PublicKey().String()), xc.NewAmountBlockchainFromUint64(10000))
			require.NoError(t, err)
			input := tx_input.NewTxInput()
			if tc.explicitConfig {
				input.TransactionConfig = &solana.TransactionConfig{}
			}
			result, err := c.WithTransferSimulation(context.Background(), args, input)
			require.Equal(t, tc.wantCalls, calls)
			if tc.wantError {
				require.ErrorContains(t, err, tc.message)
				return
			}
			require.NoError(t, err)
			require.False(t, result.(*tx_input.TxInput).SupportsV1)
			require.Equal(t, uint64(20001), result.(*tx_input.TxInput).UnitsConsumed)
			b, err := solanabuilder.NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			built, err := b.Transfer(args, result)
			require.NoError(t, err)
			require.Equal(t, solana.MessageVersionV0, built.(*tx.Tx).SolTx.Message.GetVersion())
		})
	}
}

func TestDisabledV1Call(t *testing.T) {
	cfg := xc.NewChainConfig(xc.FOGO)
	cfg.SolanaDisableV1 = true
	c := &client.Client{Asset: cfg}
	transaction := &solana.Transaction{}
	_, err := transaction.Message.SetVersion(solana.MessageVersionV1)
	require.NoError(t, err)
	_, err = c.FetchCallInput(context.Background(), &solanacall.TxCall{SolTx: transaction}, xcbuilder.CallArgs{})
	require.ErrorContains(t, err, "Solana V1 transactions are disabled for FOGO")
}

func TestTransferClientVersions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		chain           xc.NativeAsset
		testnet         bool
		simulationFails bool
	}{
		{name: "Solana V1", chain: xc.SOL},
		{name: "Solana simulation failure", chain: xc.SOL, simulationFails: true},
		{name: "FOGO mainnet V0", chain: xc.FOGO},
		{name: "FOGO testnet V0", chain: xc.FOGO, testnet: true},
		{name: "Eclipse mainnet V0", chain: xc.ES},
		{name: "Eclipse testnet V0", chain: xc.ES, testnet: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			supportsV1 := tc.chain == xc.SOL
			version := solana.MessageVersionV0
			maxVersion := float64(0)
			if supportsV1 {
				version = solana.MessageVersionV1
				maxVersion = 1
			}
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
					transaction, err := solana.TransactionFromBytes(binary)
					require.NoError(t, err)
					require.Equal(t, version, transaction.Message.GetVersion())
					if supportsV1 {
						require.Equal(t, tx_input.MaxComputeUnitLimit, *transaction.Message.TransactionConfig.ComputeUnitLimit)
						require.Equal(t, tx_input.MaxLoadedAccountsDataSizeLimit, *transaction.Message.TransactionConfig.LoadedAccountsDataSizeLimit)
					}
					var simulationError any
					if tc.simulationFails {
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
					require.Equal(t, maxVersion, opts["maxSupportedTransactionVersion"])
					require.Equal(t, "base64", opts["encoding"])
					result = map[string]any{"slot": 10, "blockTime": 123456, "version": maxVersion,
						"transaction": []string{base64.StdEncoding.EncodeToString(wire), "base64"},
						"meta":        map[string]any{"err": nil, "fee": 5000, "preBalances": []uint64{100000, 0, 1}, "postBalances": []uint64{85000, 10000, 1}}}
				default:
					t.Errorf("unexpected RPC: %s", request.Method)
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}))
			}))
			defer server.Close()
			f := factory.NewDefaultFactory()
			if tc.testnet {
				f = factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
			}
			cfg, ok := f.GetChain(tc.chain)
			require.True(t, ok)
			cfg.URL = server.URL
			c, err := client.NewClient(cfg)
			require.NoError(t, err)
			args, err := xcbuilder.NewTransferArgs(cfg.Base(), xc.Address(sender.PublicKey().String()), xc.Address(recipient.String()), xc.NewAmountBlockchainFromUint64(10000))
			require.NoError(t, err)
			input, err := c.FetchTransferInput(context.Background(), args)
			if tc.simulationFails {
				require.ErrorContains(t, err, "simulation failed")
				require.False(t, submitted)
				return
			}
			require.NoError(t, err)
			require.True(t, simulated)
			require.Equal(t, supportsV1, input.(*tx_input.TxInput).SupportsV1)
			if supportsV1 {
				require.Equal(t, uint32(24002), input.(*tx_input.TxInput).ComputeUnitLimit)
				require.Equal(t, uint32(65536), input.(*tx_input.TxInput).LoadedAccountsDataSizeLimit)
			}
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
			require.Equal(t, version, signed.Message.GetVersion())
			wire, err = built.Serialize()
			require.NoError(t, err)
			require.NoError(t, signed.VerifySignatures())
			req, err := xctypes.SubmitTxReqFromTx(tc.chain, built)
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
