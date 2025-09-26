package rpc

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockbook/types"
	"github.com/shopspring/decimal"
)

type GetBlockHashResponse struct {
	BlockHash string `json:"blockHash"`
}

type GetBlockStatsResponse struct {
	AvgFeeRate float64 `json:"avgfeerate"`
	MinFeeRate float64 `json:"minfeerate"`
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

func (client *Client) GetBlock(ctx context.Context, blockHash string) (types.Block, error) {
	var result types.Block
	params := []interface{}{blockHash, 1} // verbosity = 1 for JSON object
	err := client.call(ctx, "getblock", params, &result)
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

func (client *Client) GetRawTransaction(ctx context.Context, txid string, blockHash ...string) (GetRawTransactionResponse, error) {
	// Always use verbose = 1 for JSON object response
	params := []interface{}{txid, 1}

	// Add block hash if provided
	if len(blockHash) > 0 && blockHash[0] != "" {
		params = append(params, blockHash[0])
	}

	var result GetRawTransactionResponse
	err := client.call(ctx, "getrawtransaction", params, &result)
	if err != nil {
		return GetRawTransactionResponse{}, fmt.Errorf("failed to get raw transaction for txid %s: %w", txid, err)
	}
	return result, nil
}

func (client *Client) getRawTransactionOutput(ctx context.Context, txid string, vout int, cache map[string]GetRawTransactionResponse) (*RawTransactionOutput, error) {
	var rawTx GetRawTransactionResponse
	var err error

	if cachedTx, exists := cache[txid]; exists {
		rawTx = cachedTx
	} else {
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

func (client *Client) GetTx(ctx context.Context, txid string, chaincfg *chaincfg.Params, blockHash ...string) (types.TransactionResponse, error) {
	decimals := client.chain.Decimals
	// Get raw transaction data
	rawTx, err := client.GetRawTransaction(ctx, txid, blockHash...)
	if err != nil {
		return types.TransactionResponse{}, fmt.Errorf("failed to get raw transaction: %w", err)
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
		if false {
			// // Handle witness data (P2WPKH, P2WSH, etc.)
			// // For P2WPKH, the witness contains [signature, pubkey]
			// // For P2WSH, the witness contains [signature1, signature2, ..., script, scriptPubKey]
			// // This looks like P2WPKH - extract public key from witness
			// pubKeyHex := input.TxInWitness[len(input.TxInWitness)-1] // Last element is usually the public key
			// pubKeyBytes, _ := hex.DecodeString(pubKeyHex)
			// if len(pubKeyBytes) == 33 {
			// 	// Generate P2WPKH address from the public key
			// 	pubKeyHash := btcutil.Hash160(pubKeyBytes)
			// 	witnessAddr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, chaincfg)
			// 	if err != nil {
			// 		logrus.WithError(err).Warn("failed to derive P2WPKH address from witness")
			// 	} else {
			// 		addresses = append(addresses, witnessAddr.EncodeAddress())
			// 		fmt.Println("address:", witnessAddr.String())
			// 	}
			// }
		} else {

			scriptHex := vout.ScriptPubKey.Hex
			scriptPubKey, err := hex.DecodeString(scriptHex)
			if err != nil {
				return types.TransactionResponse{}, fmt.Errorf("failed to decode scriptPubKey hex: %w", err)
			}

			// Extract addresses from the scriptPubKey
			_, extracted, _, err := txscript.ExtractPkScriptAddrs(scriptPubKey, chaincfg)
			if err != nil {
				return types.TransactionResponse{}, fmt.Errorf("failed to extract addresses from scriptPubKey: %w", err)
			}
			// Convert addresses to strings
			for _, addr := range extracted {
				addresses = append(addresses, addr.String())
			}
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
