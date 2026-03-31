package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
)

// OwnedOutput represents an output that belongs to our wallet, with all the
// information needed to spend it.
type OwnedOutput struct {
	Amount      uint64
	TxHash      string
	OutputIndex uint64
	GlobalIndex uint64 // populated later from get_outs
	PublicKey   string // hex, the one-time output key
	Commitment  string // hex, the Pedersen commitment
	TxPubKey    string // hex, the transaction public key R (needed for spending)
	// The derivation scalar needed to compute the one-time private key for spending
	DerivationScalar []byte
	// Which subaddress this output was sent to
	SubaddressIndex crypto.SubaddressIndex
}

// ScanBlocksForOwnedOutputs scans recent blocks for outputs belonging to this wallet.
// Returns all owned outputs found within the scan range.
func (c *Client) ScanBlocksForOwnedOutputs(ctx context.Context, scanDepth uint64) ([]OwnedOutput, error) {
	privView, pubSpend, err := deriveWalletKeys()
	if err != nil {
		return nil, fmt.Errorf("cannot derive keys: %w", err)
	}
	subKeys := buildSubaddressMap(privView, pubSpend, defaultSubaddressCount)

	blockCount, err := c.getBlockCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block count: %w", err)
	}

	startHeight := blockCount - scanDepth
	if startHeight > blockCount { // underflow
		startHeight = 0
	}

	logrus.WithFields(logrus.Fields{
		"start_height": startHeight,
		"end_height":   blockCount,
	}).Info("scanning for owned outputs")

	var owned []OwnedOutput

	for height := startHeight; height < blockCount; height++ {
		blockResult, err := c.jsonRPCRequest(ctx, "get_block", map[string]interface{}{
			"height": height,
		})
		if err != nil {
			continue
		}

		var block struct {
			TxHashes []string `json:"tx_hashes"`
		}
		if err := json.Unmarshal(blockResult, &block); err != nil || len(block.TxHashes) == 0 {
			continue
		}

		const batchSize = 25
		for batchStart := 0; batchStart < len(block.TxHashes); batchStart += batchSize {
			batchEnd := batchStart + batchSize
			if batchEnd > len(block.TxHashes) {
				batchEnd = len(block.TxHashes)
			}
			batch := block.TxHashes[batchStart:batchEnd]

			txResult, err := c.httpRequest(ctx, "/get_transactions", map[string]interface{}{
				"txs_hashes":     batch,
				"decode_as_json": true,
			})
			if err != nil {
				continue
			}

			var txResp struct {
				Txs []struct {
					AsJson string `json:"as_json"`
					TxHash string `json:"tx_hash"`
				} `json:"txs"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(txResult, &txResp); err != nil || txResp.Status != "OK" {
				continue
			}

			for _, txData := range txResp.Txs {
				if txData.AsJson == "" {
					continue
				}
				outputs, err := scanTransactionForOutputs(txData.AsJson, txData.TxHash, privView, pubSpend, subKeys)
				if err != nil {
					continue
				}
				owned = append(owned, outputs...)
			}
		}
	}

	logrus.WithField("found", len(owned)).Info("scan complete")
	return owned, nil
}

// scanTransactionForOutputs scans a single tx and returns detailed owned output info.
func scanTransactionForOutputs(
	txJsonStr string,
	txHash string,
	privateViewKey, publicSpendKey []byte,
	subKeys map[crypto.SubaddressIndex][]byte,
) ([]OwnedOutput, error) {
	var txJson moneroTxJson
	if err := json.Unmarshal([]byte(txJsonStr), &txJson); err != nil {
		return nil, err
	}

	extraBytes := make([]byte, len(txJson.Extra))
	for i, v := range txJson.Extra {
		extraBytes[i] = byte(v)
	}
	txPubKey, err := crypto.ParseTxPubKey(extraBytes)
	if err != nil {
		return nil, nil
	}

	// Compute derivation once: D = 8 * viewKey * txPubKey
	derivation, err := crypto.GenerateKeyDerivation(txPubKey, privateViewKey)
	if err != nil {
		return nil, nil
	}

	var owned []OwnedOutput

	for outputIdx, vout := range txJson.Vout {
		outputKey := getOutputKey(vout)
		if outputKey == "" {
			continue
		}

		var encAmount string
		if outputIdx < len(txJson.RctSignatures.EcdhInfo) {
			encAmount = txJson.RctSignatures.EcdhInfo[outputIdx].Amount
		}

		matched, matchedIdx, amount, err := crypto.ScanOutputForSubaddresses(
			txPubKey, uint64(outputIdx), outputKey, encAmount,
			privateViewKey, publicSpendKey, subKeys,
		)
		if err != nil || !matched {
			continue
		}

		// Compute the derivation scalar for this output (needed for spending)
		scalar, _ := crypto.DerivationToScalar(derivation, uint64(outputIdx))

		// Get the commitment from rct_signatures
		commitment := ""
		// RingCT commitments are in outPk, but not always in the decoded JSON.
		// The commitment can be reconstructed from the amount and mask.

		owned = append(owned, OwnedOutput{
			Amount:           amount,
			TxHash:           txHash,
			OutputIndex:      uint64(outputIdx),
			PublicKey:        outputKey,
			Commitment:       commitment,
			TxPubKey:         hex.EncodeToString(txPubKey),
			DerivationScalar: scalar,
			SubaddressIndex:  matchedIdx,
		})

		logrus.WithFields(logrus.Fields{
			"tx_hash":      txHash,
			"output_index": outputIdx,
			"amount":       amount,
			"subaddress":   fmt.Sprintf("%d/%d", matchedIdx.Major, matchedIdx.Minor),
		}).Info("found owned output")
	}

	return owned, nil
}

// PopulateTransferInput scans for owned outputs, fetches their global indices,
// and populates decoy ring members for each output.
func (c *Client) PopulateTransferInput(ctx context.Context, input *tx_input.TxInput, from xc.Address) error {
	// Scan for our outputs
	ownedOutputs, err := c.ScanBlocksForOwnedOutputs(ctx, 200)
	if err != nil {
		return fmt.Errorf("output scanning failed: %w", err)
	}

	if len(ownedOutputs) == 0 {
		return fmt.Errorf("no spendable outputs found")
	}

	// Store the view key hex for the builder
	secret := signer.ReadPrivateKeyEnv()
	if secret != "" {
		secretBz, _ := hex.DecodeString(secret)
		_, privView, _, _, _ := crypto.DeriveKeysFromSpend(secretBz)
		input.ViewKeyHex = hex.EncodeToString(privView)
	}

	// For each owned output, we need to find its global index.
	// We do this by fetching the transaction and looking up output indices.
	for i, out := range ownedOutputs {
		// Get global output indices for this transaction
		globalIdx, commitment, err := c.getOutputGlobalIndex(ctx, out.TxHash, out.OutputIndex)
		if err != nil {
			logrus.WithError(err).WithField("tx_hash", out.TxHash).Warn("failed to get global index, skipping output")
			continue
		}
		ownedOutputs[i].GlobalIndex = globalIdx
		ownedOutputs[i].Commitment = commitment

		logrus.WithFields(logrus.Fields{
			"tx_hash":      out.TxHash,
			"output_index": out.OutputIndex,
			"global_index": globalIdx,
		}).Debug("resolved global output index")
	}

	// Fetch decoys for each output
	for _, out := range ownedOutputs {
		if out.GlobalIndex == 0 {
			continue
		}

		decoys, err := c.FetchDecoys(ctx, out.GlobalIndex, ringSize-1)
		if err != nil {
			logrus.WithError(err).Warn("failed to fetch decoys")
			continue
		}

		var ringMembers []tx_input.RingMember
		for _, d := range decoys {
			ringMembers = append(ringMembers, tx_input.RingMember{
				GlobalIndex: d.GlobalIndex,
				PublicKey:   d.PublicKey,
				Commitment:  d.Commitment,
			})
		}

		input.Outputs = append(input.Outputs, tx_input.Output{
			Amount:      out.Amount,
			Index:       out.OutputIndex,
			TxHash:      out.TxHash,
			GlobalIndex: out.GlobalIndex,
			PublicKey:   out.PublicKey,
			Commitment:  out.Commitment,
			Mask:        out.TxPubKey, // Store tx pub key in Mask field for the builder
			RingMembers: ringMembers,
		})
	}

	if len(input.Outputs) == 0 {
		return fmt.Errorf("no spendable outputs with decoys found")
	}

	return nil
}

// getOutputGlobalIndex fetches the global output index for a specific output in a transaction.
func (c *Client) getOutputGlobalIndex(ctx context.Context, txHash string, outputIndex uint64) (uint64, string, error) {
	result, err := c.httpRequest(ctx, "/get_transactions", map[string]interface{}{
		"txs_hashes":     []string{txHash},
		"decode_as_json": true,
	})
	if err != nil {
		return 0, "", err
	}

	var txResp struct {
		Txs []struct {
			OutputIndices []uint64 `json:"output_indices"`
			AsJson        string   `json:"as_json"`
		} `json:"txs"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &txResp); err != nil {
		return 0, "", err
	}
	if txResp.Status != "OK" || len(txResp.Txs) == 0 {
		return 0, "", fmt.Errorf("failed to get tx %s", txHash)
	}

	tx := txResp.Txs[0]
	if int(outputIndex) >= len(tx.OutputIndices) {
		return 0, "", fmt.Errorf("output index %d out of range (tx has %d outputs)", outputIndex, len(tx.OutputIndices))
	}

	globalIdx := tx.OutputIndices[outputIndex]

	// Also get the commitment from the rct outPk
	commitment := ""
	// Fetch commitment from get_outs
	outsResult, err := c.httpRequest(ctx, "/get_outs", map[string]interface{}{
		"outputs": []map[string]uint64{{"amount": 0, "index": globalIdx}},
	})
	if err == nil {
		var outsResp struct {
			Outs []struct {
				Key  string `json:"key"`
				Mask string `json:"mask"`
			} `json:"outs"`
			Status string `json:"status"`
		}
		if json.Unmarshal(outsResult, &outsResp) == nil && len(outsResp.Outs) > 0 {
			commitment = outsResp.Outs[0].Mask
		}
	}

	return globalIdx, commitment, nil
}
