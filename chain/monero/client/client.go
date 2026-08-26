package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"filippo.io/edwards25519"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	monerotx "github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/sirupsen/logrus"
)

type Client struct {
	url     string
	cfg     *xc.ChainConfig
	http    *http.Client
	lws     *LWSClient // light wallet server for indexed queries (required)
	viewKey []byte     // shared private view key
}

func NewClient(cfg *xc.ChainConfig) (*Client, error) {
	url, _ := cfg.ClientURL()
	if url == "" {
		return nil, fmt.Errorf("monero RPC URL not configured")
	}
	if cfg.ChainClientConfig == nil || cfg.ChainClientConfig.ViewKey == "" {
		return nil, fmt.Errorf("monero requires chain.view_key to be configured")
	}
	viewKey, err := hex.DecodeString(cfg.ChainClientConfig.ViewKey)
	if err != nil || len(viewKey) != 32 {
		return nil, fmt.Errorf("monero view_key must be 64 hex chars (32 bytes)")
	}
	if cfg.IndexerUrl == "" {
		return nil, fmt.Errorf("monero requires chain.indexer_url (monero-lws endpoint)")
	}

	c := &Client{
		url:     url,
		cfg:     cfg,
		viewKey: viewKey,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		lws: NewLWSClient(cfg.IndexerUrl),
	}
	logrus.WithField("lws_url", cfg.IndexerUrl).Info("using monero-lws for indexed queries")

	return c, nil
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
		Type     int    `json:"type"`
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

func (c *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()

	blockCount, err := c.getBlockCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block count: %w", err)
	}
	input.BlockHeight = blockCount

	// Get fee estimation via JSON-RPC
	feeResult, err := c.jsonRPCRequest(ctx, "get_fee_estimate", nil)
	if err != nil {
		logrus.WithError(err).Warn("failed to get fee estimate, using default")
		input.PerByteFee = 20000
	} else {
		var feeEstimate struct {
			Fee              uint64 `json:"fee"`
			QuantizationMask uint64 `json:"quantization_mask"`
		}
		if err := json.Unmarshal(feeResult, &feeEstimate); err != nil {
			logrus.WithError(err).Warn("failed to parse fee estimate")
			input.PerByteFee = 20000
		} else {
			input.PerByteFee = feeEstimate.Fee
			input.QuantizationMask = feeEstimate.QuantizationMask
			logrus.WithFields(logrus.Fields{
				"fee_per_byte":      feeEstimate.Fee,
				"quantization_mask": feeEstimate.QuantizationMask,
			}).Info("fee estimate")
		}
	}

	// Populate spendable outputs from the Light Wallet Server.
	// LWS is required - this implementation does not support block scanning.
	if c.lws == nil {
		return nil, fmt.Errorf("monero-lws indexer_url is required (block scanning not supported)")
	}
	if err := c.populateFromLWS(ctx, input, args.GetFrom(), args.GetAmount().Uint64()); err != nil {
		return nil, fmt.Errorf("failed to find spendable outputs: %w", err)
	}

	return input, nil
}

// populateFromLWS fetches spendable outputs from the Light Wallet Server and
// selects the ones a transfer of `amount` will spend.
// Instant - no block scanning needed.
func (c *Client) populateFromLWS(ctx context.Context, input *tx_input.TxInput, from xc.Address, amount uint64) error {
	// Use the fixed shared view key (no private spend key needed)
	privView := c.viewKey

	// Set LWS credentials and login
	c.lws.SetCredentials(string(from), hex.EncodeToString(privView))
	if err := c.lws.Login(ctx); err != nil {
		return fmt.Errorf("LWS login failed: %w", err)
	}

	// Fetch unspent outputs
	outputs, perByteFee, feeMask, err := c.lws.GetUnspentOuts(ctx)
	if err != nil {
		return fmt.Errorf("get_unspent_outs failed: %w", err)
	}

	// Use LWS fee estimates if available
	if perByteFee > 0 {
		input.PerByteFee = perByteFee
	}
	if feeMask > 0 {
		input.QuantizationMask = feeMask
	}

	// Set RNG seed
	input.RngSeed = make([]byte, 32)
	_, err = rand.Read(input.RngSeed)
	if err != nil {
		return fmt.Errorf("failed to generate RNG seed: %w", err)
	}

	// Monero enforces CRYPTONOTE_DEFAULT_TX_SPENDABLE_AGE = 10: an output must
	// be at least 10 blocks deep before it can be spent. Selecting an immature
	// output (e.g. fresh change) gets the tx rejected with `invalid_input`.
	const spendableAge = 10
	mature := outputs[:0]
	var lockedSum uint64
	for _, out := range outputs {
		if out.Height != 0 && out.Height+spendableAge > input.BlockHeight {
			logrus.WithFields(logrus.Fields{
				"height": out.Height, "tip": input.BlockHeight,
			}).Debug("skipping immature output (< 10 confirmations)")
			if v, err := out.Amount.Int64(); err == nil {
				lockedSum += uint64(v)
			}
			continue
		}
		mature = append(mature, out)
	}
	outputs = mature

	// Convert LWS outputs to tx_input format
	converted := ConvertLWSOutputs(outputs, privView)

	// Hand out only the outputs this transfer will spend: the minimum set
	// covering the amount plus the smallest outputs for dust consolidation
	// (the builder spends everything in the input). Listing the whole wallet
	// would make every subsequent transfer's input overlap with this one, so
	// IndependentOf would report a conflict forever and the next transfer
	// would queue behind an already-final transaction.
	if amount > 0 {
		selected, total := input.SelectOutputs(converted, amount)
		need := amount + input.EstimatedFeeFor(len(selected))
		if total < need {
			toHuman := func(v uint64) string {
				amt := xc.NewAmountBlockchainFromUint64(v)
				return amt.ToHuman(12).String()
			}
			if total+lockedSum >= need {
				return fmt.Errorf(
					"current balance has %s unlocked funds and %s locked funds; need to wait for more confirmations before %s can be transferred",
					toHuman(total), toHuman(lockedSum), toHuman(amount))
			}
			return fmt.Errorf(
				"insufficient funds: balance is %s (%s unlocked, %s locked), need %s plus fees",
				toHuman(total+lockedSum), toHuman(total), toHuman(lockedSum), toHuman(amount))
		}
		converted = selected
	}

	// Fetch decoys for each output
	for i := range converted {
		out := &converted[i]
		if out.GlobalIndex == 0 {
			continue
		}

		decoys, err := c.FetchDecoys(ctx, out.GlobalIndex, ringSize-1)
		if err != nil {
			logrus.WithError(err).WithField("tx_hash", out.TxHash).Warn("failed to fetch decoys")
			continue
		}

		if len(decoys) < 15 {
			logrus.WithField("tx_hash", out.TxHash).Warn("insufficient decoys")
			continue
		}

		var ringMembers []tx_input.RingMember
		for _, d := range decoys {
			pub, err := hex.DecodeString(d.PublicKey)
			if err != nil {
				logrus.WithError(err).WithField("global_index", d.GlobalIndex).Warn("skipping decoy with invalid public key")
				continue
			}
			comm, err := hex.DecodeString(d.Commitment)
			if err != nil {
				logrus.WithError(err).WithField("global_index", d.GlobalIndex).Warn("skipping decoy with invalid commitment")
				continue
			}
			ringMembers = append(ringMembers, tx_input.RingMember{
				GlobalIndex: d.GlobalIndex,
				PublicKey:   pub,
				Commitment:  comm,
			})
		}
		out.RingMembers = ringMembers
	}

	// Filter outputs with enough ring members
	for _, out := range converted {
		if len(out.RingMembers) >= 15 {
			input.Outputs = append(input.Outputs, out)
		}
	}

	if len(input.Outputs) == 0 {
		return fmt.Errorf("no spendable outputs with sufficient decoys found via LWS")
	}
	// Declare the ring size the builder must enforce across all inputs.
	input.RingSize = ringSize

	logrus.WithField("spendable", len(input.Outputs)).Info("populated outputs from LWS")
	return nil
}

func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	address := args.Address()
	if address == "" {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("address is required")
	}
	if c.lws == nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("monero-lws indexer_url is required (block scanning not supported)")
	}

	c.lws.SetCredentials(string(address), hex.EncodeToString(c.viewKey))
	if err := c.lws.Login(ctx); err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("LWS login: %w", err)
	}
	info, err := c.lws.GetAddressInfo(ctx)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("LWS get_address_info: %w", err)
	}
	received, err := strconv.ParseUint(info.TotalReceived, 10, 64)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("LWS total_received %q: %w", info.TotalReceived, err)
	}
	sent, err := strconv.ParseUint(info.TotalSent, 10, 64)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("LWS total_sent %q: %w", info.TotalSent, err)
	}
	locked, err := strconv.ParseUint(info.LockedFunds, 10, 64)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("LWS locked_funds %q: %w", info.LockedFunds, err)
	}
	// Report the true full balance, including outputs still inside the
	// 10-block spendable-age window. Guarding against spending locked funds
	// happens in FetchTransferInput, which only gathers unlocked outputs and
	// reports a clear error when the amount needs still-locked funds.
	balance := received - sent
	logrus.WithFields(logrus.Fields{
		"received": received,
		"sent":     sent,
		"locked":   locked,
		"balance":  balance,
		"scanned":  info.ScannedHeight,
	}).Info("balance from LWS")
	return xc.NewAmountBlockchainFromUint64(balance), nil
}

func (c *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 12, nil
}

// txKnown reports whether the daemon already has this transaction, either in
// the mempool or mined in a block.
func (c *Client) txKnown(ctx context.Context, txHash string) bool {
	result, err := c.httpRequest(ctx, "/get_transactions", map[string]interface{}{
		"txs_hashes": []string{txHash},
	})
	if err != nil {
		return false
	}
	var resp struct {
		Txs []struct {
			InPool      bool   `json:"in_pool"`
			BlockHeight uint64 `json:"block_height"`
		} `json:"txs"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return false
	}
	return len(resp.Txs) > 0
}

// isKeyImageSpent returns the daemon's spent status for each key image:
// 0 = unspent, 1 = spent in a block, 2 = spent in the mempool.
func (c *Client) isKeyImageSpent(ctx context.Context, keyImages []string) ([]int, error) {
	result, err := c.httpRequest(ctx, "/is_key_image_spent", map[string]interface{}{
		"key_images": keyImages,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		SpentStatus []int  `json:"spent_status"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" || len(resp.SpentStatus) != len(keyImages) {
		return nil, fmt.Errorf("is_key_image_spent returned status %q", resp.Status)
	}
	return resp.SpentStatus, nil
}

func (c *Client) SubmitTx(ctx context.Context, submitReq xctypes.SubmitTxReq, _ xcbuilder.SubmitArgs) error {
	txData := submitReq.TxData
	if len(txData) == 0 {
		return fmt.Errorf("empty transaction data")
	}

	txHex := hex.EncodeToString(txData)
	txHash, hashErr := monerotx.HashFromBlob(txData)
	if hashErr != nil {
		logrus.WithError(hashErr).Warn("could not compute tx hash from blob")
	}

	// Import authoritative key images to LWS *before* broadcasting so a
	// concurrent FetchTransferInput can't re-select an output we're consuming.
	// If the daemon definitively rejects the broadcast, we roll the pending
	// imports back below; if we crash in between, the LWS expiry window
	// eventually reverts them.
	var importedMeta *monerotx.BroadcastMetadata
	if c.lws != nil && submitReq.BroadcastInput != "" {
		var meta monerotx.BroadcastMetadata
		if err := json.Unmarshal([]byte(submitReq.BroadcastInput), &meta); err != nil {
			logrus.WithError(err).Warn("could not parse monero broadcast metadata; skipping key-image import")
		} else if meta.Sender != "" && len(meta.SpentKeyImages) > 0 {
			imgs := make([]ImportKeyImage, 0, len(meta.SpentKeyImages))
			for _, s := range meta.SpentKeyImages {
				imgs = append(imgs, ImportKeyImage{GlobalIndex: s.GlobalIndex, KeyImage: s.KeyImage})
			}
			c.lws.SetCredentials(meta.Sender, hex.EncodeToString(c.viewKey))
			if err := c.lws.ImportKeyImages(ctx, meta.Sender, imgs); err != nil {
				// Non-fatal: broadcast still proceeds; import is retryable and the
				// network is the real double-spend arbiter.
				logrus.WithError(err).Warn("failed to import key images to LWS (non-fatal)")
			} else {
				importedMeta = &meta
			}
		}
	}

	// rollbackImports un-marks the pending imports after a definitive
	// rejection so the outputs become selectable again immediately (instead of
	// waiting out the LWS expiry window). Only key images the daemon reports
	// as unspent are removed: a key image spent on-chain or in the pool is a
	// real spend (by this tx's earlier attempt or a competing tx), and
	// removing its import would make the LWS re-offer a consumed output —
	// every transfer built from it then fails as a double spend. (Relying on
	// the LWS to refuse the removal is racy: it accepts the removal until its
	// scanner has processed the spending block.)
	rollbackImports := func() {
		if importedMeta == nil {
			return
		}
		kis := make([]string, 0, len(importedMeta.SpentKeyImages))
		for _, s := range importedMeta.SpentKeyImages {
			kis = append(kis, s.KeyImage)
		}
		statuses, err := c.isKeyImageSpent(ctx, kis)
		if err != nil {
			// Can't tell what is really spent: keep all imports rather than
			// risk un-marking a real spend. The LWS expiry window reverts
			// imports for outputs that never get spent.
			logrus.WithError(err).Warn("could not check key image spent status; leaving pending imports (revert after LWS expiry)")
			return
		}
		gidxs := make([]uint64, 0, len(importedMeta.SpentKeyImages))
		for i, s := range importedMeta.SpentKeyImages {
			if statuses[i] == 0 {
				gidxs = append(gidxs, s.GlobalIndex)
			} else {
				logrus.WithFields(logrus.Fields{
					"global_index": s.GlobalIndex, "key_image": s.KeyImage,
				}).Info("keeping key image import: already spent on-chain or in pool")
			}
		}
		if err := c.lws.RemoveKeyImages(ctx, importedMeta.Sender, gidxs); err != nil {
			logrus.WithError(err).Warn("failed to roll back pending key images (outputs revert after LWS expiry)")
		}
	}

	// If the daemon already knows this exact transaction (mempool or mined), a
	// previous submit attempt succeeded: report success instead of letting the
	// daemon reject the duplicate — which would look like a failure and roll
	// back the key-image imports of a transaction that actually went through.
	if hashErr == nil && c.txKnown(ctx, string(txHash)) {
		logrus.WithField("hash", txHash).Info("transaction already in mempool or mined; treating submit as success")
		return nil
	}

	// Sanity check the inputs before broadcasting: if any of this tx's key
	// images is already spent (and the spender is not this tx — checked
	// above), a competing transaction consumed the input and this tx can
	// never confirm. Fail with a clear error instead of the daemon's opaque
	// rejection. The imports for the spent inputs are kept (they reflect the
	// real spend, so the wallet stops offering those outputs); the rest are
	// rolled back.
	if importedMeta != nil {
		kis := make([]string, 0, len(importedMeta.SpentKeyImages))
		for _, s := range importedMeta.SpentKeyImages {
			kis = append(kis, s.KeyImage)
		}
		statuses, err := c.isKeyImageSpent(ctx, kis)
		if err != nil {
			logrus.WithError(err).Warn("could not pre-check key image spent status")
		} else {
			var spentGidxs []uint64
			for i, s := range statuses {
				if s != 0 {
					spentGidxs = append(spentGidxs, importedMeta.SpentKeyImages[i].GlobalIndex)
				}
			}
			if len(spentGidxs) > 0 {
				rollbackImports()
				return fmt.Errorf(
					"inputs already spent by another transaction (output global indexes %v): the transfer input is stale, refetch it and rebuild",
					spentGidxs)
			}
		}
	}

	params := map[string]interface{}{
		"tx_as_hex":    txHex,
		"do_not_relay": false,
	}

	result, err := c.httpRequest(ctx, "/send_raw_transaction", params)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	var submitResult struct {
		Status            string `json:"status"`
		Reason            string `json:"reason"`
		DoubleSpend       bool   `json:"double_spend"`
		FeeTooLow         bool   `json:"fee_too_low"`
		InvalidInput      bool   `json:"invalid_input"`
		InvalidOutput     bool   `json:"invalid_output"`
		LowMixin          bool   `json:"low_mixin"`
		NotRelayed        bool   `json:"not_relayed"`
		Overspend         bool   `json:"overspend"`
		TooBig            bool   `json:"too_big"`
		TooFewOutputs     bool   `json:"too_few_outputs"`
		SanityCheckFailed bool   `json:"sanity_check_failed"`
	}
	if err := json.Unmarshal(result, &submitResult); err != nil {
		return fmt.Errorf("failed to parse submit result: %w (raw: %s)", err, string(result))
	}
	if submitResult.Status != "OK" {
		logrus.WithFields(logrus.Fields{
			"status":         submitResult.Status,
			"reason":         submitResult.Reason,
			"double_spend":   submitResult.DoubleSpend,
			"fee_too_low":    submitResult.FeeTooLow,
			"invalid_input":  submitResult.InvalidInput,
			"invalid_output": submitResult.InvalidOutput,
			"low_mixin":      submitResult.LowMixin,
			"overspend":      submitResult.Overspend,
			"too_big":        submitResult.TooBig,
			"sanity_failed":  submitResult.SanityCheckFailed,
		}).Error("transaction rejected by node")
		// The tx may have been accepted by another node (or an earlier attempt)
		// and mined between our idempotency check and this submit — a mined
		// tx's resubmit is rejected as a double spend of its own key images.
		if hashErr == nil && c.txKnown(ctx, string(txHash)) {
			logrus.WithField("hash", txHash).Info("transaction mined during submit; treating as success")
			return nil
		}
		// Definitive rejection: this exact tx can never confirm, so un-mark the
		// pending key-image imports — except any key image already spent
		// on-chain or in the pool, which reflects a real spend by another tx.
		rollbackImports()
		return fmt.Errorf("transaction rejected: %s (status: %s)", submitResult.Reason, submitResult.Status)
	}

	// Relay via LWS only after the authoritative daemon accepted the tx.
	// Relaying first is a split-brain hazard: the LWS's daemon can accept and
	// propagate a tx that ours rejects (e.g. a borderline relay sanity check),
	// so "rejected" txs could still confirm after we rolled back their key
	// images.
	if c.lws != nil {
		if _, err := c.lws.post(ctx, "submit_raw_tx", map[string]interface{}{
			"tx": txHex,
		}); err != nil {
			logrus.WithError(err).Debug("LWS relay failed (daemon already accepted)")
		}
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
	if !txData.InPool {
		confirmations = blockCount - txData.BlockHeight
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

	// Build TxInfo using library constructors
	block := txinfo.NewBlock(xc.XMR, txData.BlockHeight, "", time.Unix(int64(txData.BlockTimestamp), 0))
	info := txinfo.NewTxInfo(block, c.cfg.GetChain(), string(hash), confirmations, nil)
	info.Fees = []*txinfo.Balance{
		txinfo.NewBalance(xc.XMR, xc.ContractAddress(xc.XMR), fee, nil),
	}

	// Decode outputs using the fixed view key (no private spend key needed).
	// This finds ALL outputs sent to any address sharing our view key,
	// and recovers the recipient address for each.
	if txData.AsJson != "" {
		privView := c.viewKey
		addrPrefix := crypto.MainnetAddressPrefix
		if c.cfg != nil && (string(c.cfg.ChainID) == "testnet" || c.cfg.Network == "testnet") {
			addrPrefix = crypto.TestnetAddressPrefix
		}
		outputs := scanTransactionViewKeyOnly(txData.AsJson, privView, addrPrefix)
		if len(outputs) > 0 {
			// Native XMR transfer: empty contract = native asset
			mv := txinfo.NewMovement(xc.XMR, "")

			// From: total spent (sum of outputs + fee), sender hidden by ring sigs
			var totalOut uint64
			for _, out := range outputs {
				totalOut += out.amount
			}
			mv.AddSource("", xc.NewAmountBlockchainFromUint64(totalOut+txJson.RctSignatures.TxnFee), nil)

			// To: each decoded output with its recovered address
			for _, out := range outputs {
				mv.AddDestination(out.address, xc.NewAmountBlockchainFromUint64(out.amount), nil)
			}

			info.Movements = append(info.Movements, mv)
		}
	}

	return *info, nil
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

type decodedOutput struct {
	amount  uint64
	address xc.Address
}

// scanTransactionViewKeyOnly decodes output amounts using only the private view key,
// then matches each output against known spend keys to determine the recipient address.
// This is how an exchange scans for deposits across all user addresses.
func scanTransactionViewKeyOnly(txJsonStr string, privateViewKey []byte, addressPrefix byte) []decodedOutput {
	var txJson moneroTxJson
	if err := json.Unmarshal([]byte(txJsonStr), &txJson); err != nil {
		return nil
	}

	extraBytes := make([]byte, len(txJson.Extra))
	for i, v := range txJson.Extra {
		extraBytes[i] = byte(v)
	}
	txPubKey, err := crypto.ParseTxPubKey(extraBytes)
	if err != nil {
		return nil
	}

	derivation, err := crypto.GenerateKeyDerivation(txPubKey, privateViewKey)
	if err != nil {
		return nil
	}

	// Compute the public view key from the private view key
	pubView, _ := crypto.PublicFromPrivate(privateViewKey)

	var results []decodedOutput
	for outputIdx, vout := range txJson.Vout {
		outputKey := getOutputKey(vout)

		var encAmount string
		if outputIdx < len(txJson.RctSignatures.EcdhInfo) {
			encAmount = txJson.RctSignatures.EcdhInfo[outputIdx].Amount
		}
		if encAmount == "" {
			continue
		}

		scalar, err := crypto.DerivationToScalar(derivation, uint64(outputIdx))
		if err != nil {
			continue
		}

		amount, err := crypto.DecryptAmount(encAmount, scalar)
		if err != nil {
			continue
		}

		if amount == 0 || amount >= 1000000000000000000 {
			continue
		}

		// Derive the expected public spend key: pubSpend = P - H_s(D||idx)*G
		// If we can recover a valid spend key, we can reconstruct the full address.
		addr := recoverAddress(outputKey, scalar, pubView, addressPrefix)

		results = append(results, decodedOutput{amount: amount, address: addr})
	}

	return results
}

// recoverAddress recovers the recipient's Monero address from an output key.
// Given output key P and derivation scalar s: pubSpend = P - s*G
// Then address = base58(prefix || pubSpend || pubView || checksum)
func recoverAddress(outputKeyHex string, scalar []byte, pubView []byte, prefix byte) xc.Address {
	outputKeyBytes, err := hex.DecodeString(outputKeyHex)
	if err != nil || len(outputKeyBytes) != 32 {
		return ""
	}

	P, err := edwards25519.NewIdentityPoint().SetBytes(outputKeyBytes)
	if err != nil {
		return ""
	}

	sScalar, err := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	if err != nil {
		return ""
	}

	// pubSpend = P - s*G
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar)
	negSG := edwards25519.NewIdentityPoint().Negate(sG)
	pubSpend := edwards25519.NewIdentityPoint().Add(P, negSG)

	return xc.Address(crypto.GenerateAddressWithPrefix(prefix, pubSpend.Bytes(), pubView))
}

var _ xclient.Client = &Client{}
