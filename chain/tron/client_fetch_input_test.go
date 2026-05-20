package tron

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func TestRefBlockFromBlock(t *testing.T) {
	block := mockBlockResponse()

	refBlockBytes, refBlockHash, err := refBlockFromBlock(block)

	require.NoError(t, err)
	require.Equal(t, testutil.FromHex("5273"), refBlockBytes)
	require.Equal(t, testutil.FromHex("40c45983779ab5f8"), refBlockHash)
}

func TestFetchTransferInputTokenUsesLatestBlockReference(t *testing.T) {
	var createTransactionCalled atomic.Bool
	var estimateEnergyCalled atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wallet/getblockbylatestnum":
			_, _ = w.Write([]byte(`{
				"block": [{
					"blockID": "00000000015e527340c45983779ab5f83e76cc37b2f899df372709f6938d322a",
					"block_header": {"raw_data": {"number": 22958707, "timestamp": 1710000000000}}
				}]
			}`))
		case "/wallet/estimateenergy":
			estimateEnergyCalled.Store(true)
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, "TLxb4uoV9EedCe4CzPZhPqVbCEUGvuipuX", req["owner_address"])
			require.Equal(t, "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", req["contract_address"])
			require.Equal(t, TRC20_TRANSFER_FUNCTION, req["function_selector"])
			require.Equal(t, true, req["visible"])
			_, _ = w.Write([]byte(`{
				"result": {"result": true},
				"energy_required": 65000
			}`))
		case "/wallet/getchainparameters":
			_, _ = w.Write([]byte(`{
				"chainParameter": [
					{"key": "getEnergyFee", "value": 420}
				]
			}`))
		case "/wallet/createtransaction":
			createTransactionCalled.Store(true)
			http.Error(w, "unexpected native transfer simulation", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := xc.NewChainConfig(xc.TRX).
		WithDecimals(6).
		WithUrl(server.URL).
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(50))
	client, err := NewClient(cfg)
	require.NoError(t, err)

	args, err := builder.NewTransferArgs(
		cfg.Base(),
		xc.Address("TLxb4uoV9EedCe4CzPZhPqVbCEUGvuipuX"),
		xc.Address("TRU3VXqPAtyMr2RNMiZnfVXwNfJzg9oKh8"),
		xc.NewAmountBlockchainFromUint64(4_025_000_000),
		builder.OptionContractAddress("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 6),
	)
	require.NoError(t, err)

	input, err := client.FetchTransferInput(context.Background(), args)

	require.NoError(t, err)
	require.False(t, createTransactionCalled.Load())
	require.True(t, estimateEnergyCalled.Load())
	tronInput := input.(*txinput.TxInput)
	require.Equal(t, testutil.FromHex("5273"), tronInput.RefBlockBytes)
	require.Equal(t, testutil.FromHex("40c45983779ab5f8"), tronInput.RefBlockHash)
	require.Equal(t, uint64(27_300_000), tronInput.MaxFee.Uint64())
}

func TestFetchTransferInputTokenFallsBackToTriggerConstantEnergy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wallet/getblockbylatestnum":
			_, _ = w.Write([]byte(`{
				"block": [{
					"blockID": "00000000015e527340c45983779ab5f83e76cc37b2f899df372709f6938d322a",
					"block_header": {"raw_data": {"number": 22958707, "timestamp": 1710000000000}}
				}]
			}`))
		case "/wallet/estimateenergy":
			http.Error(w, "this node does not support estimate energy", http.StatusNotFound)
		case "/wallet/triggerconstantcontract":
			_, _ = w.Write([]byte(`{
				"result": {"result": true},
				"energy_used": 60000
			}`))
		case "/wallet/getchainparameters":
			_, _ = w.Write([]byte(`{
				"chainParameter": [
					{"key": "getEnergyFee", "value": 420}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := xc.NewChainConfig(xc.TRX).
		WithDecimals(6).
		WithUrl(server.URL).
		WithGasBudgetDefault(xc.NewAmountHumanReadableFromFloat(50))
	client, err := NewClient(cfg)
	require.NoError(t, err)

	args, err := builder.NewTransferArgs(
		cfg.Base(),
		xc.Address("TLxb4uoV9EedCe4CzPZhPqVbCEUGvuipuX"),
		xc.Address("TRU3VXqPAtyMr2RNMiZnfVXwNfJzg9oKh8"),
		xc.NewAmountBlockchainFromUint64(4_025_000_000),
		builder.OptionContractAddress("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 6),
	)
	require.NoError(t, err)

	input, err := client.FetchTransferInput(context.Background(), args)

	require.NoError(t, err)
	tronInput := input.(*txinput.TxInput)
	require.Equal(t, uint64(25_200_000), tronInput.MaxFee.Uint64())
}

func mockBlockResponse() *httpclient.BlockResponse {
	return &httpclient.BlockResponse{
		BlockHeader: httpclient.BlockHeader{
			RawData: httpclient.BlockHeaderRawData{
				Number: 22958707,
			},
		},
		BlockId: "00000000015e527340c45983779ab5f83e76cc37b2f899df372709f6938d322a",
	}
}
