package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/client"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (rt roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return rt(r) }

func mockClient(t *testing.T, chain string) (*client.Client, map[string]int) {
	t.Helper()
	calls := map[string]int{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Type string `json:"type"`
			User string `json:"user"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		calls[request.Type]++
		var response string
		switch request.Type {
		case "spotMeta":
			response = `{"tokens":[{"name":"USDC","tokenId":"0x6d1e7cde53ba9467b783cb7c530ce054","weiDecimals":8},{"name":"HYPE","tokenId":"0x0d01dc56dcaaca66ad901c959b4011ec","weiDecimals":8}]}`
		case "spotClearinghouseState":
			response = `{"balances":[{"coin":"USDC","total":"5.17904","hold":"1"},{"coin":"HYPE","total":"0.23867774","hold":"0"}]}`
		case "clearinghouseState":
			response = `{"marginSummary":{"totalRawUsd":"931.76"},"withdrawable":"931.76"}`
		default:
			http.Error(w, "unexpected method", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	})
	cfg := xc.NewChainConfig("HYPE")
	cfg.URL = "https://hyperliquid.invalid"
	cfg.IndexerUrl = cfg.URL
	cfg.Network = chain
	cl, err := client.NewClient(cfg)
	require.NoError(t, err)
	cl.HttpClient = &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Result(), nil
	})}
	return cl, calls
}

func TestTransferAssets(t *testing.T) {
	const spotUSDC = "USDC:0x6d1e7cde53ba9467b783cb7c530ce054"
	const spotHYPE = "HYPE:0x0d01dc56dcaaca66ad901c959b4011ec"
	for _, network := range []string{"mainnet", "testnet"} {
		t.Run(network, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				contract     xc.ContractAddress
				omitContract bool
				omitDecimals bool
				balance      string
				token        string
			}{
				{name: "perps", contract: "USDCPerps", balance: "93176000000"},
				{name: "perps default decimals", contract: "USDCPerps", omitDecimals: true, balance: "93176000000"},
				{name: "empty", omitDecimals: true, balance: "93176000000"},
				{name: "omitted", omitContract: true, omitDecimals: true, balance: "93176000000"},
				{name: "spot USDC", contract: spotUSDC, balance: "417904000", token: spotUSDC},
				{name: "spot HYPE", contract: spotHYPE, balance: "23867774", token: spotHYPE},
				{name: "USDC token ID", contract: "0x6d1e7cde53ba9467b783cb7c530ce054", balance: "417904000", token: spotUSDC},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cl, calls := mockClient(t, network)
					ctx := context.Background()
					from := xc.Address("0x216733adb4c48a741184618fe55246a568040a25")
					to := xc.Address("0xac99ec69acddad5724b310acbac8e2dceab8de79")
					balanceArgs := xclient.NewBalanceArgs(from)
					if !tc.omitContract {
						balanceArgs.SetContract(tc.contract)
					}
					balance, err := cl.FetchBalance(ctx, balanceArgs)
					require.NoError(t, err)
					require.Equal(t, tc.balance, balance.String())
					decimals, err := cl.FetchDecimals(ctx, tc.contract)
					require.NoError(t, err)
					require.Equal(t, 8, decimals)
					options := []xcbuilder.BuilderOption{}
					if !tc.omitContract {
						options = append(options, xcbuilder.OptionContractAddress(tc.contract))
					}
					if !tc.omitDecimals {
						options = append(options, xcbuilder.OptionContractDecimals(decimals))
					}
					args, err := xcbuilder.NewTransferArgs(cl.Asset.Base(), from, to, xc.NewAmountBlockchainFromUint64(93176000000), options...)
					require.NoError(t, err)
					input, err := cl.FetchTransferInput(ctx, args)
					require.NoError(t, err)
					hypeInput := input.(*tx_input.TxInput)
					require.EqualValues(t, 8, hypeInput.DecimalsOld)
					require.False(t, hypeInput.TransactionTime.IsZero())
					hypeInput.TransactionTime = time.UnixMilli(1758098505704)
					if tc.token == "" {
						require.Empty(t, hypeInput.TokenLabelOld)
						require.Equal(t, map[string]int{"clearinghouseState": 1}, calls, "perps must not depend on spot metadata")
					} else {
						require.Zero(t, calls["clearinghouseState"])
					}
					txBuilder, err := builder.NewTxBuilder(cl.Asset.Base())
					require.NoError(t, err)
					transaction, err := txBuilder.Transfer(args, input)
					require.NoError(t, err)
					hashes, err := transaction.Sighashes()
					require.NoError(t, err)
					require.Len(t, hashes, 1)
					key, err := crypto.GenerateKey()
					require.NoError(t, err)
					signature, err := crypto.Sign(hashes[0].Payload, key)
					require.NoError(t, err)
					require.NoError(t, transaction.SetSignatures(&xc.SignatureResponse{Signature: signature}))
					serialized, err := transaction.Serialize()
					require.NoError(t, err)
					var payload struct {
						Action struct {
							Type, Token, Amount, Destination, HyperliquidChain string
							Time                                               uint64
						} `json:"action"`
						Nonce     uint64 `json:"nonce"`
						Signature struct {
							R, S string
							V    int
						} `json:"signature"`
					}
					require.NoError(t, json.Unmarshal(serialized, &payload))
					expectedAction := "spotSend"
					if tc.token == "" {
						expectedAction = "usdSend"
						require.NotContains(t, string(serialized), `"token"`)
					}
					require.Equal(t, expectedAction, payload.Action.Type)
					require.Equal(t, tc.token, payload.Action.Token)
					require.Equal(t, "931.76", payload.Action.Amount)
					require.Equal(t, string(to), payload.Action.Destination)
					expectedChain := "Mainnet"
					if network == "testnet" {
						expectedChain = "Testnet"
					}
					require.Equal(t, expectedChain, payload.Action.HyperliquidChain)
					require.Equal(t, uint64(1758098505704), payload.Action.Time)
					require.Equal(t, payload.Action.Time, payload.Nonce)
					require.NotEmpty(t, payload.Signature.R)
					require.NotEmpty(t, payload.Signature.S)
					require.Contains(t, []int{27, 28}, payload.Signature.V)
				})
			}
		})
	}
}

func TestUnknownToken(t *testing.T) {
	cl, _ := mockClient(t, "mainnet")
	ctx := context.Background()
	_, err := cl.FetchBalance(ctx, xclient.NewBalanceArgs("sender", xclient.BalanceOptionContract("unknown")))
	require.ErrorContains(t, err, "missing token metadata")
	_, err = cl.FetchDecimals(ctx, "unknown")
	require.ErrorContains(t, err, "missing token metadata")
	args, err := xcbuilder.NewTransferArgs(cl.Asset.Base(), "sender", "recipient", xc.NewAmountBlockchainFromUint64(1), xcbuilder.OptionContractAddress("unknown", 8))
	require.NoError(t, err)
	_, err = cl.FetchTransferInput(ctx, args)
	require.ErrorContains(t, err, "missing token metadata")
}

func TestSpotTransferRequiresDecimals(t *testing.T) {
	cl, _ := mockClient(t, "mainnet")
	args, err := xcbuilder.NewTransferArgs(cl.Asset.Base(), "sender", "recipient", xc.NewAmountBlockchainFromUint64(1), xcbuilder.OptionContractAddress("USDC:0x6d1e7cde53ba9467b783cb7c530ce054"))
	require.NoError(t, err)
	input, err := cl.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	txBuilder, err := builder.NewTxBuilder(cl.Asset.Base())
	require.NoError(t, err)
	_, err = txBuilder.Transfer(args, input)
	require.ErrorContains(t, err, "decimals are required")
}
