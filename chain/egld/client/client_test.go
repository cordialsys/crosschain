package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/egld/client"
	"github.com/cordialsys/crosschain/chain/egld/tx"
	"github.com/cordialsys/crosschain/chain/egld/tx_input"
	"github.com/cordialsys/crosschain/chain/egld/types"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client, err := client.NewClient(xc.NewChainConfig("EGLD"))
	require.NotNil(t, client)
	require.NoError(t, err)
}

func TestFetchTransferInput(t *testing.T) {
	require := require.New(t)

	// Mock server for account and network config
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/accounts/erd1sender" {
			// Account info response
			accountResp := struct {
				Address string `json:"address"`
				Nonce   uint64 `json:"nonce"`
				Balance string `json:"balance"`
			}{
				Address: "erd1sender",
				Nonce:   42,
				Balance: "1000000000000000000",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(accountResp)
		} else if r.URL.Path == "/network/config" {
			// Network config response
			configResp := struct {
				Data struct {
					Config struct {
						ChainID        string `json:"erd_chain_id"`
						MinGasPrice    uint64 `json:"erd_min_gas_price"`
						MinGasLimit    uint64 `json:"erd_min_gas_limit"`
						GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
					} `json:"config"`
				} `json:"data"`
			}{}
			configResp.Data.Config.ChainID = "1"
			configResp.Data.Config.MinGasPrice = 1000000000
			configResp.Data.Config.MinGasLimit = 50000
			configResp.Data.Config.GasPerDataByte = 1500

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(configResp)
		}
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching transfer input for native EGLD
	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")
	amount := xc.NewAmountBlockchainFromUint64(1000000000000000000)
	args, err := xcbuilder.NewTransferArgs(cfg.GetChain().Base(), from, to, amount)
	require.NoError(err)

	input, err := egldClient.FetchTransferInput(context.Background(), args)
	require.NoError(err)
	require.NotNil(input)

	// Verify TxInput fields
	egldInput, ok := input.(*tx_input.TxInput)
	require.True(ok)
	require.Equal(uint64(42), egldInput.Nonce)
	require.Equal(uint64(50000), egldInput.GasLimit)
	require.Equal(uint64(1000000000), egldInput.GasPrice)
	require.Equal("1", egldInput.ChainID)
	require.Equal(uint32(1), egldInput.Version)
}

func TestFetchTransferInputToken(t *testing.T) {
	require := require.New(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/accounts/erd1sender" {
			accountResp := struct {
				Address string `json:"address"`
				Nonce   uint64 `json:"nonce"`
				Balance string `json:"balance"`
			}{
				Address: "erd1sender",
				Nonce:   10,
				Balance: "5000000000000000000",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(accountResp)
		} else if r.URL.Path == "/network/config" {
			configResp := struct {
				Data struct {
					Config struct {
						ChainID        string `json:"erd_chain_id"`
						MinGasPrice    uint64 `json:"erd_min_gas_price"`
						MinGasLimit    uint64 `json:"erd_min_gas_limit"`
						GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
					} `json:"config"`
				} `json:"data"`
			}{}
			configResp.Data.Config.ChainID = "1"
			configResp.Data.Config.MinGasPrice = 1000000000
			configResp.Data.Config.MinGasLimit = 50000
			configResp.Data.Config.GasPerDataByte = 1500

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(configResp)
		}
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching transfer input for ESDT token
	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")
	amount := xc.NewAmountBlockchainFromUint64(1000000)
	args, err := xcbuilder.NewTransferArgs(cfg.GetChain().Base(), from, to, amount)
	require.NoError(err)
	// Set contract for token transfer
	args.SetContract(xc.ContractAddress("USDC-c76f1f"))

	input, err := egldClient.FetchTransferInput(context.Background(), args)
	require.NoError(err)
	require.NotNil(input)

	// Verify TxInput fields - token transfers need more gas
	egldInput, ok := input.(*tx_input.TxInput)
	require.True(ok)
	require.Equal(uint64(10), egldInput.Nonce)
	// Gas limit calculated as: minGasLimit + (gasPerDataByte × dataLength)
	// For "USDC-c76f1f" (11 bytes = 22 hex) + amount 1000000 (3 bytes = 6 hex):
	// dataLength = 14 + 22 + 6 = 42
	// gasLimit = 50000 + (1500 × 42) = 113000
	require.Equal(uint64(113000), egldInput.GasLimit)
	require.Equal(uint64(1000000000), egldInput.GasPrice)
}

func TestFetchLegacyTxInput(t *testing.T) {
	require := require.New(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/accounts/erd1sender" {
			accountResp := struct {
				Address string `json:"address"`
				Nonce   uint64 `json:"nonce"`
				Balance string `json:"balance"`
			}{
				Address: "erd1sender",
				Nonce:   5,
				Balance: "2000000000000000000",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(accountResp)
		} else if r.URL.Path == "/network/config" {
			configResp := struct {
				Data struct {
					Config struct {
						ChainID        string `json:"erd_chain_id"`
						MinGasPrice    uint64 `json:"erd_min_gas_price"`
						MinGasLimit    uint64 `json:"erd_min_gas_limit"`
						GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
					} `json:"config"`
				} `json:"data"`
			}{}
			configResp.Data.Config.ChainID = "D"
			configResp.Data.Config.MinGasPrice = 1000000000
			configResp.Data.Config.MinGasLimit = 50000
			configResp.Data.Config.GasPerDataByte = 1500

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(configResp)
		}
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test legacy method
	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")

	input, err := egldClient.FetchLegacyTxInput(context.Background(), from, to)
	require.NoError(err)
	require.NotNil(input)

	egldInput, ok := input.(*tx_input.TxInput)
	require.True(ok)
	require.Equal(uint64(5), egldInput.Nonce)
	require.Equal("D", egldInput.ChainID)
}

func TestSubmitTx(t *testing.T) {
	require := require.New(t)

	// Mock server for transaction submission
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal("/transaction/send", r.URL.Path)
		require.Equal("POST", r.Method)

		// Parse request body
		var txPayload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&txPayload)
		require.NoError(err)

		// Verify required fields are present
		require.Contains(txPayload, "nonce")
		require.Contains(txPayload, "value")
		require.Contains(txPayload, "receiver")
		require.Contains(txPayload, "sender")
		require.Contains(txPayload, "gasPrice")
		require.Contains(txPayload, "gasLimit")
		require.Contains(txPayload, "chainID")
		require.Contains(txPayload, "version")
		require.Contains(txPayload, "signature")

		// Return successful response
		response := struct {
			Data struct {
				TxHash string `json:"txHash"`
			} `json:"data"`
			Error string `json:"error"`
			Code  string `json:"code"`
		}{
			Data: struct {
				TxHash string `json:"txHash"`
			}{
				TxHash: "abc123def456",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	cfg := xc.NewChainConfig("EGLD")
	cfg.URL = server.URL
	client, err := client.NewClient(cfg)
	require.NoError(err)

	// Create a signed transaction
	egldTx := &tx.Tx{
		Nonce:     42,
		Value:     "1000000000000000000",
		Receiver:  "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:    "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice:  1000000000,
		GasLimit:  50000,
		ChainID:   "1",
		Version:   1,
		Signature: "abcd1234" + strings.Repeat("00", 60), // 64-byte signature in hex
	}

	// Convert to SubmitTxReq
	submitReq, err := xctypes.SubmitTxReqFromTx("", egldTx)
	require.NoError(err)

	// Submit the transaction
	err = client.SubmitTx(context.Background(), submitReq)
	require.NoError(err)
}

func TestSubmitTxError(t *testing.T) {
	require := require.New(t)

	// Mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			Data  struct{} `json:"data"`
			Error string   `json:"error"`
			Code  string   `json:"code"`
		}{
			Error: "insufficient funds",
			Code:  "insufficientFunds",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	cfg := xc.NewChainConfig("EGLD")
	cfg.URL = server.URL
	client, err := client.NewClient(cfg)
	require.NoError(err)

	// Create a signed transaction
	egldTx := &tx.Tx{
		Nonce:     42,
		Value:     "1000000000000000000",
		Receiver:  "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:    "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice:  1000000000,
		GasLimit:  50000,
		ChainID:   "1",
		Version:   1,
		Signature: strings.Repeat("00", 64),
	}

	submitReq, err := xctypes.SubmitTxReqFromTx("", egldTx)
	require.NoError(err)

	// Submit should fail with API error
	err = client.SubmitTx(context.Background(), submitReq)
	require.Error(err)
	require.Contains(err.Error(), "insufficient funds")
}

func TestSubmitTxUnsigned(t *testing.T) {
	require := require.New(t)

	// Create an unsigned transaction
	egldTx := &tx.Tx{
		Nonce:    42,
		Value:    "1000000000000000000",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "1",
		Version:  1,
		// No signature
	}

	// Trying to create SubmitTxReq from unsigned transaction should fail
	_, err := xctypes.SubmitTxReqFromTx("", egldTx)
	require.Error(err)
	require.Contains(err.Error(), "transaction not signed")
}

func TestFetchTxInfo(t *testing.T) {
	require := require.New(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := types.Transaction{
			TxHash:        "xyz789",
			GasLimit:      50000,
			GasPrice:      1000000000,
			GasUsed:       50000,
			MiniBlockHash: "blockxyz",
			Nonce:         4,
			Receiver:      "erd1receiver",
			ReceiverShard: 1,
			Round:         4000,
			Sender:        "erd1sender",
			SenderShard:   1,
			Signature:     "sigxyz",
			Status:        "success",
			Value:         "500000000000000000",
			Fee:           "50000000000000",
			Timestamp:     1700003000,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching transaction info using new API
	args := txinfo.NewArgs(xc.TxHash("xyz789"))
	info, err := egldClient.FetchTxInfo(context.Background(), args)
	require.NoError(err)
	require.Equal("xyz789", info.Hash)
	require.GreaterOrEqual(len(info.Movements), 1) // At least one movement
}

func TestFetchBalance(t *testing.T) {
	require := require.New(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal("/address/erd1test123/balance", r.URL.Path)

		response := types.ApiResponse[types.BalanceData]{
			Data: types.BalanceData{
				Balance: "1000000000000000000", // 1 EGLD
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching balance
	address := xc.Address("erd1test123")
	args := xclient.NewBalanceArgs(address)

	balance, err := egldClient.FetchBalance(context.Background(), args)
	require.NoError(err)
	require.Equal("1000000000000000000", balance.String())
}

func TestFetchBalanceError(t *testing.T) {
	require := require.New(t)

	// Mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := types.ApiResponse[types.BalanceData]{
			Data: types.BalanceData{
				Balance: "0",
			},
			ApiError: types.ApiError{Message: "account not found", Code: "error"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching balance
	address := xc.Address("erd1invalid")
	args := xclient.NewBalanceArgs(address)

	_, err = egldClient.FetchBalance(context.Background(), args)
	require.Error(err)
	require.Contains(err.Error(), "account not found")
}

func TestFetchNativeBalance(t *testing.T) {
	require := require.New(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := types.ApiResponse[types.BalanceData]{
			Data: types.BalanceData{
				Balance: "500000000000000000", // 0.5 EGLD
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching native balance
	address := xc.Address("erd1test456")
	balance, err := egldClient.FetchNativeBalance(context.Background(), address)
	require.NoError(err)
	require.Equal("500000000000000000", balance.String())
}

func TestFetchTokenBalance(t *testing.T) {
	require := require.New(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal("/address/erd1test789/esdt/USDC-c76f1f", r.URL.Path)

		response := types.ApiResponse[types.TokenBalanceData]{
			Data: types.TokenBalanceData{
				TokenData: types.TokenData{
					TokenIdentifier: "USDC-c76f1f",
					Balance:         "1000000", // 1 USDC (6 decimals)
					Properties:      "",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching token balance
	address := xc.Address("erd1test789")
	contract := xc.ContractAddress("USDC-c76f1f")
	balance, err := egldClient.FetchTokenBalance(context.Background(), address, contract)
	require.NoError(err)
	require.Equal("1000000", balance.String())
}

func TestFetchBalanceWithContract(t *testing.T) {
	require := require.New(t)

	// Mock server that handles both native and token balance requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/address/erd1test/esdt/MEX-455c57" {
			response := types.ApiResponse[types.TokenBalanceData]{
				Data: types.TokenBalanceData{
					TokenData: types.TokenData{
						TokenIdentifier: "MEX-455c57",
						Balance:         "5000000000000000000", // 5 MEX
						Properties:      "",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching balance with contract (token balance)
	address := xc.Address("erd1test")
	contract := xc.ContractAddress("MEX-455c57")
	args := xclient.NewBalanceArgs(address, xclient.BalanceOptionContract(contract))

	balance, err := egldClient.FetchBalance(context.Background(), args)
	require.NoError(err)
	require.Equal("5000000000000000000", balance.String())
}

func TestFetchTokenBalanceError(t *testing.T) {
	require := require.New(t)

	// Mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := types.ApiResponse[types.TokenBalanceData]{
			Data: types.TokenBalanceData{
				TokenData: types.TokenData{
					Balance: "0",
				},
			},
			ApiError: types.ApiError{Message: "token not found", Code: "notFound"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching token balance with error
	address := xc.Address("erd1invalid")
	contract := xc.ContractAddress("INVALID-token")
	_, err = egldClient.FetchTokenBalance(context.Background(), address, contract)
	require.Error(err)
	require.Contains(err.Error(), "token not found")
}

func TestFetchDecimals(t *testing.T) {
	require := require.New(t)

	// Mock server for API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tokens/USDC-c76f1f" {
			response := types.TokenProperties{
				Type:       "FungibleESDT",
				Identifier: "USDC-c76f1f",
				Name:       "USD Coin",
				Ticker:     "USDC",
				Decimals:   6,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	// Create client with mock URLs (both gateway and indexer use same mock server for testing)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching decimals for token
	contract := xc.ContractAddress("USDC-c76f1f")
	decimals, err := egldClient.FetchDecimals(context.Background(), contract)
	require.NoError(err)
	require.Equal(6, decimals)
}

func TestFetchDecimalsNative(t *testing.T) {
	require := require.New(t)

	// Create client with decimals configured
	cfg := xc.NewChainConfig("EGLD")
	cfg.Decimals = 18
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Test fetching decimals for native EGLD (no contract)
	decimals, err := egldClient.FetchDecimals(context.Background(), "")
	require.NoError(err)
	require.Equal(18, decimals)
}

func TestFetchBlockMultiShard(t *testing.T) {
	require := require.New(t)

	// Mock server simulating MultiversX Indexer API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/blocks" && r.URL.Query().Get("nonce") == "12654494" {
			// Return all blocks with nonce 12654494 (one per shard)
			response := []types.Block{
				{
					Hash:      "hash_metachain",
					Nonce:     12654494,
					Shard:     4294967295, // Metachain
					Timestamp: 1770350966,
					TxCount:   0,
				},
				{
					Hash:      "hash_shard0",
					Nonce:     12654494,
					Shard:     0,
					Timestamp: 1770350965,
					TxCount:   0,
				},
				{
					Hash:      "hash_shard1",
					Nonce:     12654494,
					Shard:     1,
					Timestamp: 1770378008,
					TxCount:   1,
				},
				{
					Hash:      "hash_shard2",
					Nonce:     12654494,
					Shard:     2,
					Timestamp: 1770358892,
					TxCount:   0,
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/blocks/hash_metachain" {
			// Return full details for metachain block
			response := types.Block{
				Hash:             "hash_metachain",
				Nonce:            12654494,
				Shard:            4294967295,
				Timestamp:        1770350966,
				TxCount:          0,
				MiniBlocksHashes: []string{},
			}
			_ = json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/blocks/hash_shard0" {
			// Return full details for shard 0 block
			response := types.Block{
				Hash:             "hash_shard0",
				Nonce:            12654494,
				Shard:            0,
				Timestamp:        1770350965,
				TxCount:          0,
				MiniBlocksHashes: []string{},
			}
			_ = json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/blocks/hash_shard1" {
			// Return full details for shard 1 block (with miniblock)
			response := types.Block{
				Hash:             "hash_shard1",
				Nonce:            12654494,
				Shard:            1,
				Timestamp:        1770378008,
				TxCount:          1,
				MiniBlocksHashes: []string{"miniblock_hash_1"},
			}
			_ = json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/blocks/hash_shard2" {
			// Return full details for shard 2 block
			response := types.Block{
				Hash:             "hash_shard2",
				Nonce:            12654494,
				Shard:            2,
				Timestamp:        1770358892,
				TxCount:          0,
				MiniBlocksHashes: []string{},
			}
			_ = json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/transactions" && r.URL.Query().Get("miniBlockHash") == "miniblock_hash_1" {
			// Return transaction for miniblock
			response := []types.MiniBlockTransaction{
				{TxHash: "tx_hash_123"},
			}
			_ = json.NewEncoder(w).Encode(response)
		} else {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client (indexer URL is used for FetchBlock)
	cfg := xc.NewChainConfig("EGLD").WithUrl(server.URL).WithIndexer("", server.URL)
	egldClient, err := client.NewClient(cfg)
	require.NoError(err)

	// Fetch block
	args := xclient.AtHeight(12654494)
	blockInfo, err := egldClient.FetchBlock(context.Background(), args)
	require.NoError(err)

	// Verify the metachain block is the main block
	require.Equal("12654494", blockInfo.Height.String())
	require.Equal("hash_metachain", blockInfo.Hash)

	// Verify we have 3 sub-blocks (shards 0, 1, 2 in order)
	require.Equal(3, len(blockInfo.SubBlocks))

	// Verify shard 0 (first sub-block, no transactions)
	require.Equal("hash_shard0", blockInfo.SubBlocks[0].Hash)
	require.Equal(0, len(blockInfo.SubBlocks[0].TransactionIds))

	// Verify shard 1 (second sub-block, has the transaction)
	require.Equal("hash_shard1", blockInfo.SubBlocks[1].Hash)
	require.Equal(1, len(blockInfo.SubBlocks[1].TransactionIds))
	require.Equal("tx_hash_123", blockInfo.SubBlocks[1].TransactionIds[0])

	// Verify shard 2 (third sub-block, no transactions)
	require.Equal("hash_shard2", blockInfo.SubBlocks[2].Hash)
	require.Equal(0, len(blockInfo.SubBlocks[2].TransactionIds))
}
