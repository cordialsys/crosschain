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

// PopulateTransferInput scans for owned outputs and populates the TxInput
// with spendable outputs and fetches decoys for ring construction.
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

	// Convert owned outputs to tx_input format
	for _, out := range ownedOutputs {
		input.Outputs = append(input.Outputs, tx_input.Output{
			Amount:      out.Amount,
			Index:       out.OutputIndex,
			TxHash:      out.TxHash,
			GlobalIndex: out.GlobalIndex,
			PublicKey:   out.PublicKey,
		})
	}

	return nil
}
