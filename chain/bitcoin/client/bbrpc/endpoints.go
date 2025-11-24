package bbrpc

import (
	"context"
	"encoding/hex"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/types"
)

type GetBlockHashResponse struct {
	BlockHash string `json:"blockHash"`
}

// LatestStats returns the latest stats from the blockbook backend
func (client *Client) LatestStats(ctx context.Context) (types.StatsResponse, error) {
	var stats types.StatsResponse

	// QuickNode doesn't have a direct stats endpoint, so we'll use bb_getBlockHash to get latest block
	// and construct a minimal stats response
	params := []interface{}{"latest"}
	var blockHash GetBlockHashResponse
	err := client.call(ctx, "bb_getBlockHash", params, &blockHash)
	if err != nil {
		return stats, err
	}

	// Get block info to construct stats
	blockParams := []interface{}{blockHash.BlockHash, map[string]interface{}{"page": 1}}
	var block types.Block
	err = client.call(ctx, "bb_getBlock", blockParams, &block)
	if err != nil {
		return stats, err
	}

	// Construct minimal stats response
	stats = types.StatsResponse{
		Backend: types.BackendStats{
			Blocks:        block.Height,
			BestBlockHash: block.Hash,
		},
	}

	return stats, nil
}

// SubmitTx submits a transaction to the network using the native sendrawtransaction RPC method
func (client *Client) SubmitTx(ctx context.Context, txBytes []byte) (string, error) {
	params := []interface{}{
		hex.EncodeToString(txBytes),
		// accept all fees
		0,
	}
	var result string
	err := client.call(ctx, "sendrawtransaction", params, &result)
	if err != nil {
		return "", fmt.Errorf("failed to submit transaction: %w", err)
	}

	return result, nil
}

// ListUtxo returns unspent transaction outputs for an address
func (client *Client) ListUtxo(ctx context.Context, addr string, confirmed bool) (types.UtxoResponse, error) {
	var utxos types.UtxoResponse

	options := map[string]interface{}{}
	if confirmed {
		options["confirmed"] = true
	}

	params := []interface{}{addr, options}
	err := client.call(ctx, "bb_getUTXOs", params, &utxos)
	if err != nil {
		return nil, err
	}

	return utxos, nil
}

func (client *Client) EstimateFee(ctx context.Context, blocks int) (types.EstimateFeeResponse, error) {
	// Call the native estimatesmartfee RPC method
	params := []interface{}{blocks, "ECONOMICAL"}
	var result struct {
		Blocks  int                    `json:"blocks"`
		Feerate xc.AmountHumanReadable `json:"feerate"`
		Errors  []string               `json:"errors"`
	}

	err := client.call(ctx, "estimatesmartfee", params, &result)
	if err != nil {
		return types.EstimateFeeResponse{}, err
	}

	// Check if there were any errors in the estimation
	if len(result.Errors) > 0 {
		return types.EstimateFeeResponse{}, fmt.Errorf("fee estimation errors: %v", result.Errors)
	}

	return types.EstimateFeeResponse{
		Result: result.Feerate.String(),
	}, nil
}

// GetTx returns transaction information
func (client *Client) GetTx(ctx context.Context, txHash string) (types.TransactionResponse, error) {
	var tx types.TransactionResponse

	params := []interface{}{txHash}
	err := client.call(ctx, "bb_getTx", params, &tx)
	if err != nil {
		return types.TransactionResponse{}, err
	}

	return tx, nil
}

// GetBlock returns block information
func (client *Client) GetBlock(ctx context.Context, block uint64) (types.Block, error) {
	return client.GetBlockWithOptions(ctx, block, map[string]interface{}{"page": 1})
}

// GetBlockWithOptions returns block information with additional option specifications
func (client *Client) GetBlockWithOptions(ctx context.Context, block uint64, options map[string]interface{}) (types.Block, error) {
	var blockData types.Block

	// First get the block hash
	hashParams := []interface{}{fmt.Sprintf("%d", block)}
	var blockHash GetBlockHashResponse
	err := client.call(ctx, "bb_getBlockHash", hashParams, &blockHash)
	if err != nil {
		return types.Block{}, err
	}

	// Then get the block data
	blockParams := []interface{}{blockHash.BlockHash, options}
	err = client.call(ctx, "bb_getBlock", blockParams, &blockData)
	if err != nil {
		return types.Block{}, err
	}

	return blockData, nil
}

func (client *Client) GetAddress(ctx context.Context, address string, options map[string]interface{}) (types.AddressResponse, error) {
	var result types.AddressResponse

	params := []interface{}{address, options}
	err := client.call(ctx, "bb_getAddress", params, &result)
	if err != nil {
		return types.AddressResponse{}, err
	}

	return result, nil
}
