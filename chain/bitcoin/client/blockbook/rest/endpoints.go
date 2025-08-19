package rest

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockbook/types"
)

func (client *Client) LatestStats(ctx context.Context) (types.StatsResponse, error) {
	var stats types.StatsResponse

	err := client.get(ctx, "/api/v2", &stats)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func (client *Client) SubmitTx(ctx context.Context, txBytes []byte) (string, error) {
	var data types.SubmitResponse
	postData := hex.EncodeToString(txBytes)
	err := client.post(ctx, "/api/v2/sendtx/", "text/plain", []byte(postData), &data)
	if err != nil {
		return "", err
	}

	return data.Result, nil
}

func (client *Client) ListUtxo(ctx context.Context, addr string, confirmed bool) (types.UtxoResponse, error) {
	var data types.UtxoResponse
	url := fmt.Sprintf("api/v2/utxo/%s", addr)
	if confirmed {
		url += "?confirmed=true"
	}

	err := client.get(ctx, url, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (client *Client) EstimateFee(ctx context.Context, blocks int) (types.EstimateFeeResponse, error) {
	var data types.EstimateFeeResponse
	// fee estimate for last N blocks
	err := client.get(ctx, fmt.Sprintf("/api/v2/estimatefee/%d", blocks), &data)
	if err != nil {
		return types.EstimateFeeResponse{}, err
	}

	return data, nil
}

func (client *Client) GetTx(ctx context.Context, txHash string) (types.TransactionResponse, error) {
	var data types.TransactionResponse
	err := client.get(ctx, fmt.Sprintf("/api/v2/tx/%s", txHash), &data)
	if err != nil {
		return types.TransactionResponse{}, err
	}

	return data, nil
}

func (client *Client) GetBlock(ctx context.Context, block uint64) (types.Block, error) {
	var data types.Block
	err := client.get(ctx, fmt.Sprintf("/api/v2/block/%d", block), &data)
	if err != nil {
		return types.Block{}, err
	}

	return data, nil
}
