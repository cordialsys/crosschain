package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/egld/tx_input"
	"github.com/cordialsys/crosschain/chain/egld/types"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
)

const (
	API_KEY = "api_key"
	// Metachain shard id
	METACHAIN = 0xFFFFFFFF
)

type Client struct {
	Asset      *xc.ChainConfig
	HttpClient *http.Client
	ApiKey     string
}

var _ xclient.Client = &Client{}

func NewClient(cfgI *xc.ChainConfig) (*Client, error) {
	httpClient := cfgI.DefaultHttpClient()

	var apiKey string
	var err error
	apiKeyRef := cfgI.Auth2
	if apiKeyRef != "" {
		apiKey, err = apiKeyRef.Load()
		if err != nil {
			return nil, fmt.Errorf("could not load TON client API key: %v", err)
		}
	}

	return &Client{
		Asset:      cfgI,
		HttpClient: httpClient,
		ApiKey:     apiKey,
	}, nil
}

func (client *Client) Get(ctx context.Context, url string, response any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if client.ApiKey != "" {
		req.Header.Add(API_KEY, client.ApiKey)
	}

	resp, err := client.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func (client *Client) Post(ctx context.Context, url string, payload any, response any) error {
	var body io.Reader
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(payloadBytes)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if client.ApiKey != "" {
		req.Header.Add(API_KEY, client.ApiKey)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(responseBody))
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(responseBody, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	indexerUrl := client.Asset.IndexerUrl
	path := fmt.Sprintf("%s/accounts/%s", indexerUrl, args.GetFrom())

	var accountData types.Account
	if err := client.Get(ctx, path, &accountData); err != nil {
		return nil, fmt.Errorf("failed to fetch account info: %w", err)
	}

	var networkConfig types.NetworkConfigResponse
	path = fmt.Sprintf("%s/network/config", client.Asset.URL)
	if err := client.Get(ctx, path, &networkConfig); err != nil {
		return nil, fmt.Errorf("failed to fetch network config: %w", err)
	}

	// Calculate gas limit based on transaction type
	gasLimit := client.calculateGasLimit(&networkConfig.Data.Config, args)

	input := tx_input.NewTxInput()
	input.Nonce = accountData.Nonce
	input.GasLimit = gasLimit
	input.GasPrice = networkConfig.Data.Config.MinGasPrice
	input.ChainID = networkConfig.Data.Config.ChainID
	input.Version = 1

	return input, nil
}

func (client *Client) calculateGasLimit(config *types.NetworkConfig, args xcbuilder.TransferArgs) uint64 {
	// For native EGLD transfers, use the minimum gas limit (no data)
	// Formula: gasLimit = minGasLimit + (gasPerDataByte × dataLength)
	contract, ok := args.GetContract()
	if !ok || contract == "" {
		// Native transfer with no data field
		return config.MinGasLimit
	}

	// For ESDT token transfers, calculate actual data field size
	// Format: "ESDTTransfer@<token_hex>@<amount_hex>"
	// Example: "ESDTTransfer@555344432d636337366631662d303236623162@0f4240"

	// Calculate token identifier hex length
	tokenHexLen := len(contract) * 2 // Each byte becomes 2 hex chars

	// Calculate amount hex length
	amountBig := args.GetAmount()
	amountBytes := amountBig.Bytes()
	if len(amountBytes) == 0 {
		amountBytes = []byte{0}
	}
	amountHexLen := len(amountBytes) * 2

	// "ESDTTransfer" = 12 chars
	// "@" = 2 chars (two separators)
	// Total: 12 + 2 + tokenHexLen + amountHexLen
	dataLength := uint64(14 + tokenHexLen + amountHexLen)

	return config.MinGasLimit + (config.GasPerDataByte * dataLength)
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *Client) SubmitTx(ctx context.Context, txInput xctypes.SubmitTxReq) error {
	txData, err := txInput.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	var txPayload types.SubmitTxRequest
	if err := json.Unmarshal(txData, &txPayload); err != nil {
		return fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	path := fmt.Sprintf("%s/transaction/send", client.Asset.URL)
	var response types.SubmitTxResponse

	if err := client.Post(ctx, path, &txPayload, &response); err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	if response.HasError() {
		return fmt.Errorf("transaction submission failed: %w", &response.ApiError)
	}

	if response.Data.TxHash == "" {
		return fmt.Errorf("transaction submitted but no hash returned")
	}

	return nil
}

func (client *Client) fetchTransaction(ctx context.Context, txHash xc.TxHash) (*types.Transaction, error) {
	indexerUrl := client.Asset.IndexerUrl
	path := fmt.Sprintf("%s/transactions/%s", indexerUrl, txHash)

	var tx types.Transaction
	if err := client.Get(ctx, path, &tx); err != nil {
		return nil, fmt.Errorf("failed to fetch transaction: %w", err)
	}

	return &tx, nil
}

func (client *Client) calculateConfirmations(ctx context.Context, txRound int64) (int64, error) {
	indexerUrl := client.Asset.IndexerUrl
	path := fmt.Sprintf("%s/network/status/%d", indexerUrl, METACHAIN)

	var statusResp types.NetworkStatusResponse

	if err := client.Get(ctx, path, &statusResp); err != nil {
		return 0, fmt.Errorf("failed to fetch network status: %w", err)
	}

	currentRound := statusResp.Data.Status.CurrentRound
	if currentRound == 0 {
		return 0, fmt.Errorf("invalid current round from network status")
	}

	confirmations := currentRound - txRound
	if confirmations < 0 {
		confirmations = 0
	}

	return confirmations, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHash := args.TxHash()
	tx, err := client.fetchTransaction(ctx, txHash)
	if err != nil {
		return txinfo.TxInfo{}, err
	}

	chainCfg := client.Asset.GetChain()

	// Calculate confirmations
	var confirmations uint64
	if tx.Round > 0 {
		confs, err := client.calculateConfirmations(ctx, int64(tx.Round))
		if err == nil && confs >= 0 {
			confirmations = uint64(confs)
		}
	}

	// Determine error status
	var errorMsg *string
	switch tx.Status {
	case "fail", "invalid":
		msg := fmt.Sprintf("transaction status: %s", tx.Status)
		errorMsg = &msg
	case "success", "pending":
		// No error
	default:
		msg := fmt.Sprintf("unknown transaction status: %s", tx.Status)
		errorMsg = &msg
	}

	// Create TxInfo
	blockTime := time.Unix(tx.Timestamp, 0).UTC()
	txInfo := txinfo.NewTxInfo(
		txinfo.NewBlock(chainCfg.Chain, uint64(tx.Round), tx.MiniBlockHash, blockTime),
		chainCfg,
		string(txHash),
		confirmations,
		errorMsg,
	)

	// Process operations or fallback to simple transfer
	if len(tx.Operations) > 0 {
		for i, op := range tx.Operations {
			if op.Action == "transfer" {
				amount := xc.NewAmountBlockchainFromStr(op.Value)
				contract := xc.ContractAddress("")
				variant := txinfo.MovementVariantNative

				if op.Identifier != "" {
					contract = xc.ContractAddress(op.Identifier)
					variant = txinfo.MovementVariantToken
				}

				from := xc.Address(op.Sender)
				to := xc.Address(op.Receiver)

				if from != "" && to != "" {
					movement := txInfo.AddSimpleTransfer(from, to, contract, amount, nil, "")
					movement.AddEventMeta(txinfo.NewEventFromIndex(uint64(i), variant))
				}
			}
		}
	} else {
		value := xc.NewAmountBlockchainFromStr(tx.Value)
		if value.Uint64() > 0 {
			from := xc.Address(tx.Sender)
			to := xc.Address(tx.Receiver)
			txInfo.AddSimpleTransfer(from, to, xc.ContractAddress(""), value, nil, "")
		}
	}

	// Add fee
	fee := xc.NewAmountBlockchainFromStr(tx.Fee)
	if fee.Uint64() > 0 {
		txInfo.AddFee(xc.Address(tx.Sender), xc.ContractAddress(""), fee, nil)
	}

	txInfo.Fees = txInfo.CalculateFees()
	txInfo.SyncDeprecatedFields()

	return *txInfo, nil
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, errors.New("not implemented")
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	if contract, ok := args.Contract(); ok {
		return client.FetchTokenBalance(ctx, args.Address(), contract)
	}
	return client.FetchNativeBalance(ctx, args.Address())
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	var balanceResp types.BalanceResponse
	path := fmt.Sprintf("%s/address/%s/balance", client.Asset.URL, address)

	if err := client.Get(ctx, path, &balanceResp); err != nil {
		return zero, err
	}

	if balanceResp.HasError() {
		return zero, &balanceResp.ApiError
	}

	balance := xc.NewAmountBlockchainFromStr(balanceResp.Data.Balance)
	return balance, nil
}

func (client *Client) FetchTokenBalance(ctx context.Context, address xc.Address, contract xc.ContractAddress) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	var tokenResp types.TokenBalanceResponse
	path := fmt.Sprintf("%s/address/%s/esdt/%s", client.Asset.URL, address, contract)

	if err := client.Get(ctx, path, &tokenResp); err != nil {
		return zero, err
	}

	if tokenResp.HasError() {
		return zero, &tokenResp.ApiError
	}

	balance := xc.NewAmountBlockchainFromStr(tokenResp.Data.TokenData.Balance)
	return balance, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if contract == "" {
		return int(client.Asset.GetChain().Decimals), nil
	}
	return client.FetchTokenDecimals(ctx, contract)
}

func (client *Client) FetchTokenDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	indexerUrl := client.Asset.IndexerUrl
	path := fmt.Sprintf("%s/tokens/%s", indexerUrl, contract)

	var tokenProps types.TokenProperties
	if err := client.Get(ctx, path, &tokenProps); err != nil {
		return 0, fmt.Errorf("failed to fetch token properties: %w", err)
	}

	return tokenProps.Decimals, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	indexerUrl := client.Asset.IndexerUrl

	// Determine which block to fetch
	height, hasHeight := args.Height()

	if !hasHeight {
		return nil, fmt.Errorf("latest block fetching not yet supported for EGLD")
	}

	// Fetch all blocks with this nonce (one per shard)
	path := fmt.Sprintf("%s/blocks?nonce=%d", indexerUrl, height)

	var blocks []types.Block
	if err := client.Get(ctx, path, &blocks); err != nil {
		return nil, fmt.Errorf("failed to fetch blocks by nonce: %w", err)
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("block not found for nonce %d", height)
	}

	// Helper function to fetch full block details and transactions
	fetchBlockWithTransactions := func(hash string) (*types.Block, []string, error) {
		// Fetch full block details to get miniBlocksHashes
		blockPath := fmt.Sprintf("%s/blocks/%s", indexerUrl, hash)

		var fullBlock types.Block
		if err := client.Get(ctx, blockPath, &fullBlock); err != nil {
			return nil, nil, fmt.Errorf("failed to fetch block details: %w", err)
		}

		txIds := []string{}

		// Fetch transactions for each miniblock
		for _, miniBlockHash := range fullBlock.MiniBlocksHashes {
			txPath := fmt.Sprintf("%s/transactions?miniBlockHash=%s", indexerUrl, miniBlockHash)

			var txs []types.MiniBlockTransaction
			if err := client.Get(ctx, txPath, &txs); err != nil {
				// Log error but continue - some miniblocks might be cross-shard
				continue
			}

			for _, tx := range txs {
				txIds = append(txIds, tx.TxHash)
			}
		}

		return &fullBlock, txIds, nil
	}

	// Sort blocks so metachain comes first, then shards in order
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].Shard == METACHAIN {
			return true
		}
		if blocks[j].Shard == METACHAIN {
			return false
		}
		return blocks[i].Shard < blocks[j].Shard
	})

	// Use metachain block as main block
	mainBlockSummary := blocks[0]
	mainBlock, mainTxIds, err := fetchBlockWithTransactions(mainBlockSummary.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch main block details: %w", err)
	}

	blockTime := time.Unix(mainBlock.Timestamp, 0)
	result := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			mainBlock.Nonce,
			mainBlock.Hash,
			blockTime,
		),
		TransactionIds: mainTxIds,
		SubBlocks:      []*txinfo.SubBlockWithTransactions{},
	}

	// Add shard blocks as sub-blocks
	for i := 1; i < len(blocks); i++ {
		shardBlockSummary := blocks[i]
		shardBlock, shardTxIds, err := fetchBlockWithTransactions(shardBlockSummary.Hash)
		if err != nil {
			// Log error but continue
			continue
		}

		shardTime := time.Unix(shardBlock.Timestamp, 0)
		subBlock := &txinfo.SubBlockWithTransactions{
			Block: *txinfo.NewBlock(
				client.Asset.GetChain().Chain,
				shardBlock.Nonce,
				shardBlock.Hash,
				shardTime,
			),
			TransactionIds: shardTxIds,
		}
		result.SubBlocks = append(result.SubBlocks, subBlock)
	}

	return result, nil
}
