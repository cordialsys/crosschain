package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
)

type Client struct {
	url  string
	cfg  *xc.ChainConfig
	http *http.Client
}

func NewClient(cfg *xc.ChainConfig) (*Client, error) {
	url, _ := cfg.ClientURL()
	if url == "" {
		return nil, fmt.Errorf("monero RPC URL not configured")
	}

	return &Client{
		url: url,
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// jsonRPCRequest makes a JSON-RPC call to the Monero daemon
func (c *Client) jsonRPCRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "0",
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := strings.TrimRight(c.url, "/") + "/json_rpc"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RPC request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w (body: %s)", err, string(respBody))
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// httpRequest makes a direct HTTP request to a Monero daemon endpoint
func (c *Client) httpRequest(ctx context.Context, path string, params interface{}) (json.RawMessage, error) {
	bodyBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := strings.TrimRight(c.url, "/") + path
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return json.RawMessage(respBody), nil
}

// getBlockCount returns the current block height
func (c *Client) getBlockCount(ctx context.Context) (uint64, error) {
	result, err := c.jsonRPCRequest(ctx, "get_block_count", nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Count uint64 `json:"count"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, err
	}
	return resp.Count, nil
}

// deriveWalletKeys loads the private key from env and derives the full key set.
// Returns (privateViewKey, publicSpendKey, error).
// The address parameter is used to verify we're scanning the right wallet (main address or subaddress).
func deriveWalletKeys() (privView, pubSpend []byte, err error) {
	secret := signer.ReadPrivateKeyEnv()
	if secret == "" {
		return nil, nil, fmt.Errorf("XC_PRIVATE_KEY not set - required for Monero view key scanning")
	}

	secretBz, err := hex.DecodeString(secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	_, privViewKey, pubSpendKey, _, err := crypto.DeriveKeysFromSpend(secretBz)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive keys: %w", err)
	}

	return privViewKey, pubSpendKey, nil
}

// defaultSubaddressCount is the number of subaddresses to precompute for scanning.
// An exchange would set this to the number of user addresses generated.
const defaultSubaddressCount = 100

// buildSubaddressMap precomputes subaddress spend keys for scanning.
func buildSubaddressMap(privView, pubSpend []byte, count uint32) map[crypto.SubaddressIndex][]byte {
	subKeys := make(map[crypto.SubaddressIndex][]byte, count)
	for i := uint32(1); i <= count; i++ {
		idx := crypto.SubaddressIndex{Major: 0, Minor: i}
		subSpend, _, err := crypto.DeriveSubaddressKeys(privView, pubSpend, idx)
		if err != nil {
			continue
		}
		subKeys[idx] = subSpend
	}
	return subKeys
}

// moneroTxJson represents the parsed JSON of a Monero transaction
type moneroTxJson struct {
	Version    int    `json:"version"`
	UnlockTime uint64 `json:"unlock_time"`
	Vin        []struct {
		Key struct {
			Amount     uint64   `json:"amount"`
			KeyOffsets []uint64 `json:"key_offsets"`
			KImage     string   `json:"k_image"`
		} `json:"key"`
	} `json:"vin"`
	Vout []struct {
		Amount uint64 `json:"amount"`
		Target struct {
			TaggedKey struct {
				Key     string `json:"key"`
				ViewTag string `json:"view_tag"`
			} `json:"tagged_key"`
			Key string `json:"key"`
		} `json:"target"`
	} `json:"vout"`
	Extra         []int `json:"extra"`
	RctSignatures struct {
		Type      int    `json:"type"`
		TxnFee   uint64 `json:"txnFee"`
		EcdhInfo []struct {
			Amount string `json:"amount"`
		} `json:"ecdhInfo"`
	} `json:"rct_signatures"`
}

// getOutputKey extracts the output one-time public key from a transaction output
func getOutputKey(vout struct {
	Amount uint64 `json:"amount"`
	Target struct {
		TaggedKey struct {
			Key     string `json:"key"`
			ViewTag string `json:"view_tag"`
		} `json:"tagged_key"`
		Key string `json:"key"`
	} `json:"target"`
}) string {
	if vout.Target.TaggedKey.Key != "" {
		return vout.Target.TaggedKey.Key
	}
	return vout.Target.Key
}

// scanTransaction scans a single transaction for outputs belonging to the given wallet,
// including both the main address and all precomputed subaddresses.
// Returns the total amount received in this transaction.
func scanTransaction(txJsonStr string, privateViewKey, publicSpendKey []byte, subKeys map[crypto.SubaddressIndex][]byte) (uint64, error) {
	var txJson moneroTxJson
	if err := json.Unmarshal([]byte(txJsonStr), &txJson); err != nil {
		return 0, fmt.Errorf("failed to parse tx JSON: %w", err)
	}

	// Extract tx public key from extra field
	extraBytes := make([]byte, len(txJson.Extra))
	for i, v := range txJson.Extra {
		extraBytes[i] = byte(v)
	}
	txPubKey, err := crypto.ParseTxPubKey(extraBytes)
	if err != nil {
		logrus.WithError(err).Debug("failed to parse tx pub key from extra")
		return 0, nil
	}

	var totalReceived uint64
	for outputIdx, vout := range txJson.Vout {
		outputKey := getOutputKey(vout)
		if outputKey == "" {
			continue
		}

		// Get encrypted amount from ecdh info
		var encryptedAmount string
		if outputIdx < len(txJson.RctSignatures.EcdhInfo) {
			encryptedAmount = txJson.RctSignatures.EcdhInfo[outputIdx].Amount
		}

		// Scan against main address + all subaddresses
		matched, matchedIdx, amount, err := crypto.ScanOutputForSubaddresses(
			txPubKey,
			uint64(outputIdx),
			outputKey,
			encryptedAmount,
			privateViewKey,
			publicSpendKey,
			subKeys,
		)
		if err != nil {
			logrus.WithError(err).WithField("output_index", outputIdx).Debug("error scanning output")
			continue
		}
		if matched {
			logrus.WithFields(logrus.Fields{
				"output_index":    outputIdx,
				"amount":          amount,
				"subaddress_major": matchedIdx.Major,
				"subaddress_minor": matchedIdx.Minor,
			}).Info("found owned output")
			totalReceived += amount
		}
	}

	return totalReceived, nil
}

func (c *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()

	blockCount, err := c.getBlockCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block count: %w", err)
	}
	input.BlockHeight = blockCount

	// Get fee estimation
	feeResult, err := c.httpRequest(ctx, "/get_fee_estimate", nil)
	if err != nil {
		logrus.WithError(err).Warn("failed to get fee estimate, using default")
		input.PerByteFee = 1000
	} else {
		var feeEstimate struct {
			Fee              uint64 `json:"fee"`
			QuantizationMask uint64 `json:"quantization_mask"`
			Status           string `json:"status"`
		}
		if err := json.Unmarshal(feeResult, &feeEstimate); err != nil {
			logrus.WithError(err).Warn("failed to parse fee estimate")
			input.PerByteFee = 1000
		} else {
			input.PerByteFee = feeEstimate.Fee
			input.QuantizationMask = feeEstimate.QuantizationMask
		}
	}

	return input, nil
}

func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	address := args.Address()
	if address == "" {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("address is required")
	}

	// Derive view key from our private key to scan outputs
	privView, pubSpend, err := deriveWalletKeys()
	if err != nil {
		logrus.WithError(err).Warn("cannot derive view key for balance scanning")
		return xc.NewAmountBlockchainFromUint64(0), nil
	}

	// Precompute subaddress spend keys for scanning
	subKeys := buildSubaddressMap(privView, pubSpend, defaultSubaddressCount)

	blockCount, err := c.getBlockCount(ctx)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to get block count: %w", err)
	}

	// Scan the last 200 blocks for outputs belonging to us.
	// This is a practical limit for detecting recent deposits.
	// A full wallet scan would require scanning from genesis.
	scanDepth := uint64(200)
	startHeight := blockCount - scanDepth
	if startHeight > blockCount { // underflow check
		startHeight = 0
	}

	logrus.WithFields(logrus.Fields{
		"start_height": startHeight,
		"end_height":   blockCount,
		"scan_depth":   scanDepth,
	}).Info("scanning blocks for Monero outputs")

	var totalBalance uint64

	// Scan blocks in batches
	for height := startHeight; height < blockCount; height++ {
		// Get block at this height
		blockResult, err := c.jsonRPCRequest(ctx, "get_block", map[string]interface{}{
			"height": height,
		})
		if err != nil {
			logrus.WithError(err).WithField("height", height).Debug("failed to get block")
			continue
		}

		var block struct {
			BlockHeader struct {
				Height uint64 `json:"height"`
			} `json:"block_header"`
			Json     string   `json:"json"`
			TxHashes []string `json:"tx_hashes"`
		}
		if err := json.Unmarshal(blockResult, &block); err != nil {
			continue
		}

		if len(block.TxHashes) == 0 {
			continue
		}

		// Fetch transactions in batches (public nodes limit requests in restricted mode)
		const batchSize = 25
		for batchStart := 0; batchStart < len(block.TxHashes); batchStart += batchSize {
			batchEnd := batchStart + batchSize
			if batchEnd > len(block.TxHashes) {
				batchEnd = len(block.TxHashes)
			}
			batch := block.TxHashes[batchStart:batchEnd]

			txParams := map[string]interface{}{
				"txs_hashes":     batch,
				"decode_as_json": true,
			}
			txResult, err := c.httpRequest(ctx, "/get_transactions", txParams)
			if err != nil {
				logrus.WithError(err).WithField("height", height).Debug("failed to get transactions")
				continue
			}

			var txResp struct {
				Txs []struct {
					AsJson string `json:"as_json"`
					TxHash string `json:"tx_hash"`
				} `json:"txs"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(txResult, &txResp); err != nil {
				continue
			}
			if txResp.Status != "OK" {
				logrus.WithField("status", txResp.Status).WithField("height", height).Debug("get_transactions returned non-OK status")
				continue
			}

			for _, tx := range txResp.Txs {
				if tx.AsJson == "" {
					continue
				}
				amount, err := scanTransaction(tx.AsJson, privView, pubSpend, subKeys)
				if err != nil {
					logrus.WithError(err).WithField("tx_hash", tx.TxHash).Debug("error scanning transaction")
					continue
				}
				if amount > 0 {
					logrus.WithFields(logrus.Fields{
						"tx_hash": tx.TxHash,
						"amount":  amount,
						"height":  height,
					}).Info("found incoming transfer")
					totalBalance += amount
				}
			}
		}
	}

	logrus.WithField("total_balance", totalBalance).Info("scan complete")
	return xc.NewAmountBlockchainFromUint64(totalBalance), nil
}

func (c *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 12, nil
}

func (c *Client) SubmitTx(ctx context.Context, submitReq xctypes.SubmitTxReq) error {
	txData := submitReq.TxData
	if len(txData) == 0 {
		return fmt.Errorf("empty transaction data")
	}

	params := map[string]interface{}{
		"tx_as_hex":    hex.EncodeToString(txData),
		"do_not_relay": false,
	}

	result, err := c.httpRequest(ctx, "/send_raw_transaction", params)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	var submitResult struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(result, &submitResult); err != nil {
		return fmt.Errorf("failed to parse submit result: %w", err)
	}
	if submitResult.Status != "OK" {
		return fmt.Errorf("transaction rejected: %s", submitResult.Reason)
	}

	return nil
}

func (c *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	hash := args.TxHash()

	params := map[string]interface{}{
		"txs_hashes":     []string{string(hash)},
		"decode_as_json": true,
	}

	result, err := c.httpRequest(ctx, "/get_transactions", params)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to fetch transaction: %w", err)
	}

	var txResult struct {
		Txs []struct {
			AsHex          string   `json:"as_hex"`
			AsJson         string   `json:"as_json"`
			BlockHeight    uint64   `json:"block_height"`
			BlockTimestamp uint64   `json:"block_timestamp"`
			TxHash         string   `json:"tx_hash"`
			InPool         bool     `json:"in_pool"`
			OutputIndices  []uint64 `json:"output_indices"`
		} `json:"txs"`
		Status   string   `json:"status"`
		MissedTx []string `json:"missed_tx"`
	}
	if err := json.Unmarshal(result, &txResult); err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to parse transaction data: %w", err)
	}
	if txResult.Status != "OK" {
		return txinfo.TxInfo{}, fmt.Errorf("get_transactions returned status: %s", txResult.Status)
	}
	if len(txResult.MissedTx) > 0 {
		return txinfo.TxInfo{}, fmt.Errorf("transaction not found: %s", hash)
	}
	if len(txResult.Txs) == 0 {
		return txinfo.TxInfo{}, fmt.Errorf("no transaction data returned for: %s", hash)
	}

	txData := txResult.Txs[0]

	blockCount, err := c.getBlockCount(ctx)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get block count: %w", err)
	}

	var confirmations uint64
	state := txinfo.Succeeded
	final := false
	if txData.InPool {
		confirmations = 0
		state = txinfo.Mining
	} else {
		confirmations = blockCount - txData.BlockHeight
		if confirmations >= uint64(c.cfg.XConfirmationsFinal) {
			final = true
		}
	}

	// Parse fee from tx JSON
	var txJson struct {
		RctSignatures struct {
			TxnFee uint64 `json:"txnFee"`
		} `json:"rct_signatures"`
	}
	if txData.AsJson != "" {
		if err := json.Unmarshal([]byte(txData.AsJson), &txJson); err != nil {
			logrus.WithError(err).Warn("failed to parse transaction JSON")
		}
	}

	fee := xc.NewAmountBlockchainFromUint64(txJson.RctSignatures.TxnFee)

	// Try to decode outputs using view key if available
	var movements []*txinfo.Movement
	secret := signer.ReadPrivateKeyEnv()
	if secret != "" && txData.AsJson != "" {
		secretBz, err := hex.DecodeString(secret)
		if err == nil {
			_, privView, pubSpend, pubView, err := crypto.DeriveKeysFromSpend(secretBz)
			if err == nil {
				myAddr := xc.Address(crypto.GenerateAddress(pubSpend, pubView))
				subKeys := buildSubaddressMap(privView, pubSpend, defaultSubaddressCount)
				amount, err := scanTransaction(txData.AsJson, privView, pubSpend, subKeys)
				if err == nil && amount > 0 {
					movements = append(movements, &txinfo.Movement{
						To: []*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(amount),
								AddressId: myAddr,
							},
						},
					})
				}
			}
		}
	}

	info := txinfo.TxInfo{
		Name:      txinfo.TransactionName(fmt.Sprintf("chains/XMR/transactions/%s", hash)),
		Hash:      string(hash),
		State:     state,
		Final:     final,
		Movements: movements,
		Fees: []*txinfo.Balance{
			txinfo.NewBalance(xc.XMR, "", fee, nil),
		},
		Block:         txinfo.NewBlock(xc.XMR, txData.BlockHeight, "", time.Unix(int64(txData.BlockTimestamp), 0)),
		Confirmations: confirmations,
	}

	return info, nil
}

func (c *Client) FetchLegacyTxInfo(ctx context.Context, hash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	args := txinfo.NewArgs(hash)
	info, err := c.FetchTxInfo(ctx, args)
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}
	return txinfo.LegacyTxInfo{
		TxID:          info.Hash,
		Confirmations: int64(info.Confirmations),
	}, nil
}

func (c *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	var result json.RawMessage
	var err error

	height, hasHeight := args.Height()
	if hasHeight {
		result, err = c.jsonRPCRequest(ctx, "get_block", map[string]interface{}{
			"height": height,
		})
	} else {
		result, err = c.jsonRPCRequest(ctx, "get_last_block_header", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block: %w", err)
	}

	var blockResult struct {
		BlockHeader struct {
			Height    uint64 `json:"height"`
			Timestamp uint64 `json:"timestamp"`
			Hash      string `json:"hash"`
		} `json:"block_header"`
	}
	if err := json.Unmarshal(result, &blockResult); err != nil {
		return nil, fmt.Errorf("failed to parse block: %w", err)
	}

	header := blockResult.BlockHeader
	block := txinfo.NewBlock(xc.XMR, header.Height, header.Hash, time.Unix(int64(header.Timestamp), 0))

	return &txinfo.BlockWithTransactions{
		Block: *block,
	}, nil
}

var _ xclient.Client = &Client{}

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no transaction") {
		return errors.TransactionNotFound
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return errors.NetworkError
	}
	return ""
}
