package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/txscript"
	"github.com/sirupsen/logrus"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/types"
	zecaddress "github.com/cordialsys/crosschain/chain/zcash/address"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/shopspring/decimal"
)

type GetBlockHashResponse struct {
	BlockHash string `json:"blockHash"`
}

type GetBlockStatsResponse struct {
	AvgFeeRate xc.AmountHumanReadable `json:"avgfeerate"`
	MinFeeRate xc.AmountHumanReadable `json:"minfeerate"`
}

type EstimateSmartFeeResponse struct {
	Feerate xc.AmountHumanReadable `json:"feerate"`
	Blocks  int                    `json:"blocks"`
}

type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

type RawTransactionInput struct {
	TxID        string    `json:"txid"`
	Vout        int       `json:"vout"`
	ScriptSig   ScriptSig `json:"scriptSig"`
	Sequence    uint32    `json:"sequence"`
	TxInWitness []string  `json:"txinwitness,omitempty"`
}

type ScriptPubKey struct {
	Asm          string   `json:"asm"`
	Hex          string   `json:"hex"`
	ReqSigs      int      `json:"reqSigs"`
	Type         string   `json:"type"`
	Addresses    []string `json:"addresses"`
	AddressMaybe string   `json:"address,omitempty"`
}

type RawTransactionOutput struct {
	Value        xc.AmountHumanReadable `json:"value"`
	N            int                    `json:"n"`
	ScriptPubKey ScriptPubKey           `json:"scriptPubKey"`
}

type GetRawTransactionResponse struct {
	TxID          string                 `json:"txid"`
	Hash          string                 `json:"hash"`
	Version       int                    `json:"version"`
	Size          int                    `json:"size"`
	LockTime      uint32                 `json:"locktime"`
	Vin           []RawTransactionInput  `json:"vin"`
	Vout          []RawTransactionOutput `json:"vout"`
	Hex           string                 `json:"hex"`
	BlockHash     string                 `json:"blockhash,omitempty"`
	Confirmations int                    `json:"confirmations,omitempty"`
	Time          int64                  `json:"time,omitempty"`
	BlockTime     int64                  `json:"blocktime,omitempty"`
}

func (client *Client) GetBlockStats(ctx context.Context) (GetBlockStatsResponse, error) {
	var stats GetBlockStatsResponse

	var blockHash string
	err := client.call(ctx, "getbestblockhash", []interface{}{}, &blockHash)
	if err != nil {
		return stats, err
	}

	statsParams := []interface{}{blockHash, []string{"minfeerate", "avgfeerate"}}
	err = client.call(ctx, "getblockstats", statsParams, &stats)
	if err != nil {
		return stats, err
	}
	return stats, nil
}

func (client *Client) EstimateFee(ctx context.Context, blocks int) (types.FeeEstimationResult, error) {
	chain := client.chain.GetChain()
	var result EstimateSmartFeeResponse
	params := []interface{}{blocks}
	err := client.call(ctx, "estimatesmartfee", params, &result)
	if err != nil {
		// Some backends do not support fee estimation
		logrus.WithError(err).Info("estimatesmartfee is not supported, trying to use native blockstats endpoint")
		// try using the native blockstats endpoint to use the avg fee rate
		stats, err := client.GetBlockStats(ctx)
		if err != nil {
			// use the configured default price, if it's set.
			defaultPrice := client.chain.GetChain().ChainGasPriceDefault
			logrus.WithField("chain", chain.Chain).
				WithField("defaultPrice", defaultPrice).
				WithField("error", err).
				Warn("using default fee price since estimate-fee is not supported")
			if client.chain.GetChain().ChainGasPriceDefault >= 1 {
				return types.FeeEstimationResult{}, nil
			}
			return types.FeeEstimationResult{}, fmt.Errorf("could not estimate fee: %v", err)
		}
		avg := stats.AvgFeeRate
		if avg.IsZero() {
			avg = xc.NewAmountHumanReadableFromFloat(1)
		}
		return types.FeeEstimationResult{
			Type: types.FeeEstimationAverage,
			Fee:  avg,
		}, nil
	}

	return types.FeeEstimationResult{
		Type: types.FeeEstimationPerKb,
		Fee:  result.Feerate,
	}, nil
}

func (client *Client) SubmitTx(ctx context.Context, txBytes []byte) (string, error) {
	params := []interface{}{
		hex.EncodeToString(txBytes),
		// 2nd param is boolean for DOGE, but numeric for other bitcoin chains, so we just omit it (taking the default).
	}
	var result string
	err := client.call(ctx, "sendrawtransaction", params, &result)
	if err != nil && strings.Contains(err.Error(), "transaction already exists in mempool") {
		return "", errors.TransactionExistsf("%v", err)
	}
	if err != nil {
		return "", fmt.Errorf("failed to submit transaction: %w", err)
	}

	return result, nil
}

func (client *Client) LatestBlock(ctx context.Context) (uint64, error) {
	bestBlockHash, err := client.GetBestBlockHash(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get best block hash: %w", err)
	}

	header, err := client.GetBlockHeader(ctx, bestBlockHash)
	if err != nil {
		return 0, fmt.Errorf("failed to get block header: %w", err)
	}

	return uint64(header.Height), nil
}

func (client *Client) GetBlock(ctx context.Context, height uint64) (types.Block, error) {
	blockHash, err := client.GetBlockHash(ctx, int(height))
	if err != nil {
		return types.Block{}, err
	}

	var result types.Block
	params := []interface{}{blockHash, 1} // verbosity = 1 for JSON object
	err = client.call(ctx, "getblock", params, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (client *Client) GetBestBlockHash(ctx context.Context) (string, error) {
	var blockHash string
	err := client.call(ctx, "getbestblockhash", []interface{}{}, &blockHash)
	if err != nil {
		return "", fmt.Errorf("failed to get best block hash: %w", err)
	}
	return blockHash, nil
}

func (client *Client) GetBlockHash(ctx context.Context, height int) (string, error) {
	var blockHash string
	params := []interface{}{height}
	err := client.call(ctx, "getblockhash", params, &blockHash)
	if err != nil {
		return "", fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}
	return blockHash, nil
}

func (client *Client) GetBlockHeader(ctx context.Context, blockHash string) (types.BlockHeader, error) {
	// Return JSON object
	params := []interface{}{blockHash, true}

	var result types.BlockHeader
	err := client.call(ctx, "getblockheader", params, &result)
	if err != nil {
		return result, fmt.Errorf("failed to get block header for hash %s: %w", blockHash, err)
	}
	return result, nil
}

func (client *Client) GetRawTransaction(ctx context.Context, txid string) (GetRawTransactionResponse, error) {
	// Always use verbose = 1 for JSON object response
	params := []interface{}{txid, 1}

	var result GetRawTransactionResponse
	err := client.call(ctx, "getrawtransaction", params, &result)
	if err != nil {
		return GetRawTransactionResponse{}, err
	}
	return result, nil
}

func (client *Client) getRawTransactionOutput(ctx context.Context, txid string, vout int, cache map[string]GetRawTransactionResponse) (*RawTransactionOutput, error) {
	var rawTx GetRawTransactionResponse
	var err error

	if cachedTx, exists := cache[txid]; exists {
		rawTx = cachedTx
	} else {
		if err := client.chain.Limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait failed on getting raw transaction %s: %w", txid, err)
		}
		rawTx, err = client.GetRawTransaction(ctx, txid)
		if err != nil {
			return nil, fmt.Errorf("failed to get raw transaction %s: %w", txid, err)
		}
		cache[txid] = rawTx
	}

	if vout >= len(rawTx.Vout) {
		return nil, fmt.Errorf("vout index %d out of range for transaction %s", vout, txid)
	}

	// Get the scriptPubKey hex from the output
	voutPoint := rawTx.Vout[vout]
	return &voutPoint, nil

	// return scriptPubKey, nil
}

func (client *Client) GetTx(ctx context.Context, txid string) (types.TransactionResponse, error) {
	decimals := client.chain.Decimals
	// Get raw transaction data
	rawTx, err := client.GetRawTransaction(ctx, txid)
	if err != nil {
		return types.TransactionResponse{}, err
	}

	// Create cache for raw transactions to avoid redundant RPC calls
	cache := make(map[string]GetRawTransactionResponse)
	// Cache the current transaction
	cache[txid] = rawTx

	// Convert Vin inputs with address extraction
	vin := make([]types.Vin, len(rawTx.Vin))
	for i, input := range rawTx.Vin {
		// Get the scriptPubKey from the previous transaction's output
		vout, err := client.getRawTransactionOutput(ctx, input.TxID, input.Vout, cache)
		if err != nil {
			return types.TransactionResponse{}, fmt.Errorf("failed to get utxo for raw transaction output: %w", err)
		}

		addresses := []string{}

		scriptHex := vout.ScriptPubKey.Hex
		scriptPubKey, err := hex.DecodeString(scriptHex)
		if err != nil {
			return types.TransactionResponse{}, fmt.Errorf("failed to decode scriptPubKey hex: %w", err)
		}

		// Extract addresses from the scriptPubKey
		_, extracted, _, err := txscript.ExtractPkScriptAddrs(scriptPubKey, client.chaincfg)
		if err != nil {
			return types.TransactionResponse{}, fmt.Errorf("failed to extract addresses from scriptPubKey: %w", err)
		}
		// Convert addresses to strings
		for _, addr := range extracted {
			if client.chain.Chain == xc.ZEC {
				tadr := zecaddress.TransparentAddress{
					Hash:         [20]byte(addr.ScriptAddress()),
					NetID:        client.chaincfg.PubKeyHashAddrID,
					ScriptHashId: client.chaincfg.ScriptHashAddrID,
				}
				addresses = append(addresses, tadr.EncodeAddress())
			} else {
				addresses = append(addresses, addr.String())
			}
			addresses = append(addresses, addr.String())
		}
		amount := vout.Value.ToBlockchain(decimals)

		vin[i] = types.Vin{
			TxID:      input.TxID,
			Vout:      input.Vout,
			Sequence:  input.Sequence,
			N:         i,
			Addresses: addresses,
			IsAddress: len(addresses) > 0,
			Value:     amount,
			Hex:       input.ScriptSig.Hex,
		}
	}

	// Convert Vout outputs
	vout := make([]types.Vout, len(rawTx.Vout))
	totalValue := decimal.NewFromFloat(0)
	for i, output := range rawTx.Vout {
		addresses := output.ScriptPubKey.Addresses
		if output.ScriptPubKey.AddressMaybe != "" {
			addresses = []string{output.ScriptPubKey.AddressMaybe}
		}
		vout[i] = types.Vout{
			Value:     output.Value.ToBlockchain(decimals),
			N:         output.N,
			Hex:       output.ScriptPubKey.Hex,
			Addresses: addresses,
			IsAddress: len(addresses) > 0,
		}
		totalValue = totalValue.Add(output.Value.Decimal())
	}

	blockHeight := 0
	// may not be in a block yet
	if rawTx.BlockHash != "" {
		blockHeader, err := client.GetBlockHeader(ctx, rawTx.BlockHash)
		if err != nil {
			return types.TransactionResponse{}, fmt.Errorf("failed to get block header: %w", err)
		}
		blockHeight = blockHeader.Height
	}

	response := types.TransactionResponse{
		TxID:          rawTx.TxID,
		Version:       rawTx.Version,
		Vin:           vin,
		Vout:          vout,
		BlockHash:     rawTx.BlockHash,
		BlockHeight:   blockHeight,
		Confirmations: rawTx.Confirmations,
		BlockTime:     rawTx.BlockTime,
		Size:          rawTx.Size,
		Vsize:         rawTx.Size,
		Value:         totalValue.String(),
		ValueIn:       "0",
		Fees:          "0",
		Hex:           rawTx.Hex,
	}

	return response, nil
}

func (client *Client) ListUtxo(ctx context.Context, addr string, confirmed bool) (types.UtxoResponse, error) {
	return client.bbClient.ListUtxo(ctx, addr, confirmed)
}
