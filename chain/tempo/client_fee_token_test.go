package tempo

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestFetchFeeTokenTransferInput(t *testing.T) {
	chain, args, _ := feeTokenFixture(t)
	var estimate map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var result any
		switch request.Method {
		case "eth_getTransactionCount":
			var account string
			if err := json.Unmarshal(request.Params[0], &account); err != nil {
				t.Error(err)
			}
			if !strings.EqualFold(account, string(testSender)) {
				t.Errorf("unexpected nonce lookup for %s", account)
			}
			result = "0x7"
		case "eth_chainId":
			result = "0xa5bf"
		case "eth_getBlockByNumber":
			result = &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(0),
				BaseFee: big.NewInt(20_000_000_000), GasLimit: 30_000_000}
		case "eth_maxPriorityFeePerGas":
			result = "0x0"
		case "txpool_contentFrom":
			result = map[string]any{"pending": map[string]any{}, "queued": map[string]any{}}
		case "eth_estimateGas":
			if err := json.Unmarshal(request.Params[0], &estimate); err != nil {
				t.Error(err)
			}
			result = "0xf4240"
		default:
			t.Errorf("unexpected RPC method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewClient(chain.WithUrl(server.URL))
	require.NoError(t, err)
	defer client.EthClient.Close()
	input, err := client.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	require.Equal(t, "0x76", estimate["type"])
	require.Equal(t, string(testFeeToken), estimate["feeToken"])
	require.Equal(t, string(testSender), estimate["from"])
	require.Equal(t, "0x7", estimate["nonce"])
	require.Equal(t, "0x0", estimate["nonceKey"])
	calls := estimate["calls"].([]any)
	require.Len(t, calls, 1)
	require.Equal(t, string(testTransferToken), strings.ToLower(calls[0].(map[string]any)["to"].(string)))
	require.Equal(t, "0x0", calls[0].(map[string]any)["value"])
	require.Contains(t, calls[0].(map[string]any)["input"], "a9059cbb")
	require.Equal(t, uint64(1_001_000), input.(*TxInput).GasLimit)
	fee, contract := input.GetFeeLimit()
	require.Equal(t, testFeeToken, contract)
	require.Equal(t, "20020", fee.String())

	// An offline builder receives the same fee currency after input serialization.
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	var offline TxInput
	require.NoError(t, json.Unmarshal(encoded, &offline))
	b, err := NewTxBuilder(chain.Base())
	require.NoError(t, err)
	tx, err := b.Transfer(args, &offline)
	require.NoError(t, err)
	require.IsType(t, &FeeTokenTx{}, tx)

	// Sponsored transfers use only the sender nonce and a configured gas budget.
	args.SetFeePayer(testFeePayer)
	estimate = nil
	chain.GasLimitDefault = 750_000
	sponsored, err := client.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	require.Nil(t, estimate, "do not simulate sponsorship as a sender-paid transaction")
	require.Equal(t, testFeePayer, sponsored.(*TxInput).FeePayerAddress)
	require.True(t, sponsored.(*TxInput).NativeFeePayer)
	require.Zero(t, sponsored.(*TxInput).FeePayerNonce)
	require.Equal(t, uint64(750_000), sponsored.(*TxInput).GasLimit)
	require.False(t, sponsored.IsFeeLimitAccurate())
	fee, contract = sponsored.GetFeeLimit()
	require.Equal(t, testFeeToken, contract)
	require.Equal(t, "15000", fee.String())
	encoded, err = json.Marshal(sponsored)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &offline))
	_, err = b.Transfer(args, &offline)
	require.NoError(t, err)
	badArgs, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient,
		xc.NewAmountBlockchainFromUint64(1), xcbuilder.OptionContractAddress(testTransferToken),
		xcbuilder.OptionFeeContract("bad"))
	require.NoError(t, err)
	_, err = client.FetchTransferInput(context.Background(), badArgs)
	require.ErrorContains(t, err, "valid fee token contract")
}
