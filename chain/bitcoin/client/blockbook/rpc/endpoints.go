package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
)

type GetBlockHashResponse struct {
	BlockHash string `json:"blockHash"`
}

type GetBlockStatsResponse struct {
	AvgFeeRate float64 `json:"avgfeerate"`
	MinFeeRate float64 `json:"minfeerate"`
}

// GetBlockStats returns the latest stats using the getblockstats RPC method
func (client *Client) GetBlockStats(ctx context.Context) (GetBlockStatsResponse, error) {
	var stats GetBlockStatsResponse

	// First, get the latest block hash using getbestblockhash
	var blockHash string
	err := client.call(ctx, "getbestblockhash", []interface{}{}, &blockHash)
	if err != nil {
		return stats, err
	}

	// Get block stats using getblockstats RPC method
	statsParams := []interface{}{blockHash, []string{"minfeerate", "avgfeerate"}}
	err = client.call(ctx, "getblockstats", statsParams, &stats)
	if err != nil {
		return stats, err
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
