package client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	nearbuilder "github.com/cordialsys/crosschain/chain/near/builder"
	"github.com/cordialsys/crosschain/chain/near/client"
	"github.com/cordialsys/crosschain/chain/near/tx"
	"github.com/cordialsys/crosschain/chain/near/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	bin "github.com/gagliardetto/binary"
	"github.com/stretchr/testify/require"
)

func TestTransferRegistration(t *testing.T) {
	const minimum = "1250000000000000000000"
	const bounds = `{"min":"` + minimum + `","max":null}`
	for _, tt := range []struct {
		gasFloor                                     string
		name, balance, bounds, rpcFailure, wantError string
		native, register                             bool
		multiplier                                   float64
	}{
		{name: "unregistered", balance: `null`, bounds: bounds, register: true},
		{name: "registered with no available storage", balance: `{"total":"` + minimum + `","available":"0"}`},
		{name: "registered with zero total", balance: `{"total":"0","available":"0"}`},
		{name: "legacy deposit ignored", balance: `{"total":"1","available":"0"}`},
		{name: "discount cannot underfund gas", balance: `null`, bounds: bounds, register: true, multiplier: 0.5},
		{name: "RPC admission gas floor", balance: `null`, bounds: bounds, register: true, gasFloor: "1000000000"},
		{name: "free registration", balance: `null`, bounds: `{"min":"0","max":"0"}`, register: true},
		{name: "native", native: true},
		{name: "malformed storage", balance: `{`, wantError: "unmarshal storage balance"},
		{name: "empty storage response", balance: ``, wantError: "unmarshal storage balance"},
		{name: "token balance is not storage", balance: `"100"`, wantError: "unmarshal storage balance"},
		{name: "missing storage fields", balance: `{}`, wantError: "invalid storage total"},
		{name: "bad minimum", balance: `null`, bounds: `{"min":"invalid"}`, wantError: "invalid minimum"},
		{name: "negative minimum", balance: `null`, bounds: `{"min":"-1"}`, wantError: "invalid minimum"},
		{name: "overflow minimum", balance: `null`, bounds: `{"min":"340282366920938463463374607431768211456"}`, wantError: "invalid minimum"},
		{name: "missing bounds", balance: `null`, bounds: `null`, wantError: "invalid minimum"},
		{name: "storage RPC error", rpcFailure: "storage_balance_of", wantError: "fetch storage balance"},
		{name: "bounds RPC error", balance: `null`, rpcFailure: "storage_balance_bounds", wantError: "fetch storage balance bounds"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			publicKey := make([]byte, 32)
			blockHash := base58.Encode(make([]byte, 32))
			type request struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			requests := make(chan request, 10)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				requests <- req
				var params struct {
					RequestType string `json:"request_type"`
					MethodName  string `json:"method_name"`
				}
				_ = json.Unmarshal(req.Params, &params)
				if tt.rpcFailure != "" && params.MethodName == tt.rpcFailure {
					_, _ = io.WriteString(w, `{"jsonrpc":"2.0","error":{"name":"HANDLER_ERROR","cause":"view call failed"}}`)
					return
				}
				var result any
				switch {
				case params.RequestType == "view_access_key_list":
					result = map[string]any{"block_hash": blockHash, "keys": []any{map[string]any{
						"public_key": "ed25519:" + base58.Encode(publicKey), "access_key": map[string]any{"nonce": 7, "permission": "FullAccess"},
					}}}
				case params.MethodName == "storage_balance_of", params.MethodName == "storage_balance_bounds":
					value := tt.balance
					if params.MethodName == "storage_balance_bounds" {
						value = tt.bounds
					}
					// NEAR returns contract JSON as an array of byte values, not base64.
					bytes := make([]int, len(value))
					for i := range value {
						bytes[i] = int(value[i])
					}
					result = map[string]any{"result": bytes}
				case req.Method == "EXPERIMENTAL_protocol_config":
					result = json.RawMessage(`{"min_gas_price":"` + tt.gasFloor + `","runtime_config":{"transaction_costs":{
						"action_receipt_creation_config":{"execution":100,"send_sir":20,"send_not_sir":30},
						"action_creation_config":{
							"transfer_cost":{"execution":200},
							"function_call_cost":{"execution":300,"send_sir":40,"send_not_sir":50},
							"function_call_cost_per_byte":{"execution":1,"send_sir":2,"send_not_sir":3}
						}}}}`)
				case req.Method == "gas_price":
					result = map[string]any{"gas_price": "100000000"}
				default:
					http.Error(w, "unexpected RPC", 400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result})
			}))
			defer server.Close()
			cfg := xc.NewChainConfig(xc.NEAR)
			cfg.URL = server.URL
			cfg.ChainGasMultiplier = tt.multiplier
			c, err := client.NewClient(cfg)
			require.NoError(t, err)
			options := []xcbuilder.BuilderOption{buildertest.OptionPublicKey(publicKey)}
			if !tt.native {
				options = append(options, buildertest.OptionContractAddress("token.near"))
			}
			args := buildertest.MustNewTransferArgs(cfg.Base(), "sender.near", "receiver.near", xc.NewAmountBlockchainFromUint64(123), options...)
			input, err := c.FetchTransferInput(context.Background(), args)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Nil(t, input)
				return
			}
			require.NoError(t, err)
			// Exercise the online-client/offline-builder JSON boundary.
			encoded, err := json.Marshal(input)
			require.NoError(t, err)
			var restored tx_input.TxInput
			require.NoError(t, json.Unmarshal(encoded, &restored))
			if tt.name == "legacy deposit ignored" {
				restored.RequiredDepopsit = xc.NewAmountBlockchainFromStr(minimum)
			}
			require.Equal(t, uint64(8), restored.Nonce)
			require.Equal(t, blockHash, restored.BlockHash)
			builder, err := nearbuilder.NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			transaction, err := builder.Transfer(args, &restored)
			require.NoError(t, err)
			fee, asset := restored.GetFeeLimit()
			require.Empty(t, asset)
			require.True(t, restored.IsFeeLimitAccurate())
			if tt.native {
				require.IsType(t, &tx.Tx[tx.TransferAction]{}, transaction)
				require.Len(t, requests, 3)
				require.Nil(t, restored.StorageDeposit)
				require.Equal(t, "60000000000", fee.String())
				return
			}
			require.Equal(t, "1", restored.TransferDeposit.String())
			calls := transaction.(*tx.Tx[tx.FunctionCallAction])
			require.Equal(t, "sender.near", calls.Transaction.SignerID)
			require.Equal(t, "token.near", calls.Transaction.ReceiverID)
			wantActions := 1
			if tt.register {
				wantActions = 2
				require.NotNil(t, restored.StorageDeposit)
				deposit := minimum
				if tt.name == "free registration" {
					deposit = "0"
				}
				require.Equal(t, deposit, restored.StorageDeposit.String())
				registration := calls.Transaction.Actions[0]
				require.Equal(t, "storage_deposit", registration.MethodName)
				require.JSONEq(t, `{"account_id":"receiver.near","registration_only":true}`, string(registration.Args))
				// Decode the little-endian u128 independently of the action encoder.
				bz := append([]byte(nil), registration.Deposit[:]...)
				for i, j := 0, len(bz)-1; i < j; i, j = i+1, j-1 {
					bz[i], bz[j] = bz[j], bz[i]
				}
				require.Equal(t, deposit, new(big.Int).SetBytes(bz).String())
			} else {
				require.Nil(t, restored.StorageDeposit)
			}
			require.Len(t, calls.Transaction.Actions, wantActions)
			transfer := calls.Transaction.Actions[wantActions-1]
			require.Equal(t, "ft_transfer", transfer.MethodName)
			require.JSONEq(t, `{"receiver_id":"receiver.near","amount":"123"}`, string(transfer.Args))
			require.Equal(t, [16]byte{1}, transfer.Deposit)
			gas := uint64(130)
			for _, action := range calls.Transaction.Actions {
				require.Equal(t, uint8(2), action.Type)
				require.Equal(t, uint64(30_000_000_000_000), action.Gas)
				gas += action.Gas + 350 + uint64(len(action.MethodName)+len(action.Args))*4
			}
			feeMultiplier := uint64(2)
			if tt.multiplier == 0.5 {
				feeMultiplier = 1
			}
			wantFee := xc.NewAmountBlockchainFromUint64(gas * feeMultiplier)
			gasPrice := xc.NewAmountBlockchainFromUint64(100000000)
			if tt.gasFloor != "" {
				gasPrice = xc.NewAmountBlockchainFromStr(tt.gasFloor)
			}
			wantFee = wantFee.Mul(&gasPrice)
			require.Equal(t, wantFee.String(), restored.FeeEstimation.String())
			one := xc.NewAmountBlockchainFromUint64(1)
			wantFee = wantFee.Add(&one)
			if tt.register {
				wantFee = wantFee.Add(restored.StorageDeposit)
			}
			require.Equal(t, wantFee.String(), fee.String())
			serialized, err := calls.Serialize()
			require.NoError(t, err)
			var decoded tx.Tx[tx.FunctionCallAction]
			require.NoError(t, bin.NewBorshDecoder(serialized).Decode(&decoded))
			require.Equal(t, calls.Transaction, decoded.Transaction)
			sighashes, err := calls.Sighashes()
			require.NoError(t, err)
			require.Len(t, sighashes, 1)
			require.Len(t, requests, 3+wantActions)
			for len(requests) > 0 {
				req := <-requests
				if req.Method != "query" {
					continue
				}
				var params map[string]string
				require.NoError(t, json.Unmarshal(req.Params, &params))
				if params["request_type"] != "call_function" {
					require.Equal(t, "sender.near", params["account_id"])
					continue
				}
				require.Equal(t, "token.near", params["account_id"])
				require.Equal(t, "final", params["finality"])
				body, err := base64.StdEncoding.DecodeString(params["args_base64"])
				require.NoError(t, err)
				switch params["method_name"] {
				case "storage_balance_of":
					require.JSONEq(t, `{"account_id":"receiver.near"}`, string(body))
				case "storage_balance_bounds":
					require.True(t, tt.register)
					require.JSONEq(t, `{}`, string(body))
				default:
					t.Fatalf("unexpected view method %q", params["method_name"])
				}
			}
		})
	}
}

func TestFetchBalanceRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","error":{"name":"UNKNOWN_ACCOUNT","cause":"account does not exist","data":{"account_id":"missing.near"}}}`)
	}))
	defer server.Close()
	cfg := xc.NewChainConfig(xc.NEAR)
	cfg.URL = server.URL
	c, err := client.NewClient(cfg)
	require.NoError(t, err)
	args := xclient.NewBalanceArgs("missing.near")
	_, err = c.FetchBalance(context.Background(), args)
	require.ErrorContains(t, err, "UNKNOWN_ACCOUNT")
	require.ErrorContains(t, err, "account_id:missing.near")
}
