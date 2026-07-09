package tools

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	monerotx "github.com/cordialsys/crosschain/chain/monero/tx"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdMonero() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "monero",
		Short:        "Utilities for Monero (XMR) chain",
		SilenceUsage: true,
	}
	cmd.AddCommand(CmdMoneroBackfillKeyImages())
	return cmd
}

// CmdMoneroBackfillKeyImages computes the true key image for every output the
// wallet has received (visible via LWS), checks which are actually spent
// on-chain via the daemon's /is_key_image_spent, and imports the spent ones to
// the LWS /import_key_images endpoint. This backfills authoritative
// spent-output tracking for wallets whose spends predate key-image imports.
//
// Only key images confirmed spent on-chain are imported: importing a key image
// for an unspent output would mark it pending and freeze it from selection
// until the import expiry window passes.
func CmdMoneroBackfillKeyImages() *cobra.Command {
	var privateKeyRef string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "backfill-key-images",
		Short: "Compute key images for the wallet's outputs and import the spent ones to monero-lws.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			if chainConfig.Driver != xc.DriverMonero {
				return fmt.Errorf("this command only supports monero chains (got %s)", chainConfig.Chain)
			}
			viewKey := chainConfig.ChainClientConfig.ViewKey
			if viewKey == "" {
				return fmt.Errorf("chain.view_key must be configured")
			}
			lwsUrl := chainConfig.IndexerUrl
			if lwsUrl == "" {
				return fmt.Errorf("chain.indexer_url (monero-lws) must be configured")
			}
			daemonUrl := chainConfig.URL
			if daemonUrl == "" {
				return fmt.Errorf("chain.url (monerod) must be configured")
			}

			privateKeyInput, err := config.GetSecret(privateKeyRef)
			if err != nil || privateKeyInput == "" {
				return fmt.Errorf("could not get private key (set env %s): %v", signer.EnvPrivateKey, err)
			}
			xcSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, xcaddress.OptionViewKey(viewKey))
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := xcSigner.PublicKey()
			if err != nil {
				return fmt.Errorf("could not derive public key: %v", err)
			}
			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), xcaddress.OptionViewKey(viewKey))
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}
			address, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			logrus.WithField("address", address).Info("backfilling key images")

			httpClient := &http.Client{Timeout: 30 * time.Second}
			post := func(baseUrl, endpoint string, body interface{}) (json.RawMessage, error) {
				bz, err := json.Marshal(body)
				if err != nil {
					return nil, err
				}
				resp, err := httpClient.Post(baseUrl+endpoint, "application/json", bytes.NewReader(bz))
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				respBz, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, err
				}
				if resp.StatusCode != 200 {
					return nil, fmt.Errorf("%s returned %d: %s", endpoint, resp.StatusCode, string(respBz))
				}
				return respBz, nil
			}

			// Ensure the account is registered, then list every output LWS knows.
			if _, err := post(lwsUrl, "/login", map[string]interface{}{
				"address": string(address), "view_key": viewKey,
				"create_account": true, "generated_locally": false,
			}); err != nil {
				return fmt.Errorf("LWS login: %v", err)
			}
			unspentBz, err := post(lwsUrl, "/get_unspent_outs", map[string]interface{}{
				"address": string(address), "view_key": viewKey,
				"amount": "0", "mixin": 0, "use_dust": true, "dust_threshold": "0",
			})
			if err != nil {
				return fmt.Errorf("LWS get_unspent_outs: %v", err)
			}
			var unspent struct {
				Outputs []struct {
					GlobalIndex json.Number `json:"global_index"`
					PublicKey   string      `json:"public_key"`
					TxPubKey    string      `json:"tx_pub_key"`
					Index       uint64      `json:"index"`
					Amount      json.Number `json:"amount"`
				} `json:"outputs"`
			}
			if err := json.Unmarshal(unspentBz, &unspent); err != nil {
				return fmt.Errorf("parse unspent outs: %v", err)
			}
			if len(unspent.Outputs) == 0 {
				fmt.Println("no outputs to check")
				return nil
			}

			// Compute the true key image for each output (phase-1 signing request).
			type kiEntry struct {
				GlobalIndex uint64
				Amount      string
				KeyImage    string
			}
			var entries []kiEntry
			var keyImagesHex []string
			for _, out := range unspent.Outputs {
				gidx, err := out.GlobalIndex.Int64()
				if err != nil || gidx == 0 {
					continue
				}
				payload := monerotx.EncodeSighash(&monerotx.MoneroSighash{
					TxPubKey:    out.TxPubKey,
					OutputIndex: out.Index,
				})
				sig, err := xcSigner.Sign(&xc.SignatureRequest{Payload: payload})
				if err != nil {
					return fmt.Errorf("key image derivation failed for output %d: %v", gidx, err)
				}
				if len(sig.Signature) != 32 {
					return fmt.Errorf("expected 32-byte key image for output %d, got %d bytes", gidx, len(sig.Signature))
				}
				ki := hex.EncodeToString(sig.Signature)
				entries = append(entries, kiEntry{uint64(gidx), out.Amount.String(), ki})
				keyImagesHex = append(keyImagesHex, ki)
			}

			// Ask the daemon which key images are already spent on-chain.
			spentBz, err := post(daemonUrl, "/is_key_image_spent", map[string]interface{}{
				"key_images": keyImagesHex,
			})
			if err != nil {
				return fmt.Errorf("daemon is_key_image_spent: %v", err)
			}
			var spentResp struct {
				SpentStatus []int  `json:"spent_status"`
				Status      string `json:"status"`
			}
			if err := json.Unmarshal(spentBz, &spentResp); err != nil {
				return fmt.Errorf("parse is_key_image_spent: %v", err)
			}
			if len(spentResp.SpentStatus) != len(entries) {
				return fmt.Errorf("daemon returned %d statuses for %d key images", len(spentResp.SpentStatus), len(entries))
			}

			var toImport []map[string]interface{}
			for i, entry := range entries {
				// 0 = unspent, 1 = spent on chain, 2 = spent in mempool
				spent := spentResp.SpentStatus[i] != 0
				fmt.Printf("output gidx=%d amount=%s key_image=%s spent=%v\n",
					entry.GlobalIndex, entry.Amount, entry.KeyImage, spent)
				if spent {
					toImport = append(toImport, map[string]interface{}{
						"global_index": entry.GlobalIndex,
						"key_image":    entry.KeyImage,
					})
				}
			}
			if len(toImport) == 0 {
				fmt.Println("nothing to import: no outputs are spent")
				return nil
			}
			if dryRun {
				fmt.Printf("dry-run: would import %d key image(s)\n", len(toImport))
				return nil
			}

			importBz, err := post(lwsUrl, "/import_key_images", map[string]interface{}{
				"address": string(address), "view_key": viewKey, "images": toImport,
			})
			if err != nil {
				return fmt.Errorf("LWS import_key_images: %v", err)
			}
			fmt.Printf("imported: %s\n", string(importBz))
			return nil
		},
	}
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only report which key images would be imported")
	return cmd
}
