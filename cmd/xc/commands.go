package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdRpcBalance() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance <address>",
		Short: "Check balance of an asset.  Reported as big integer, not accounting for any decimals.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			addressRaw := args[0]

			contract, _ := cmd.Flags().GetString("contract")
			cli, err := xcFactory.NewClient(assetConfig(chain, contract, 0))
			if err != nil {
				return err
			}

			address := xcFactory.MustAddress(chain, addressRaw)
			balance, err := cli.FetchBalance(context.Background(), address)
			if err != nil {
				return fmt.Errorf("could not fetch balance for address %s: %v", address, err)
			}

			fmt.Println(balance.String())
			return nil
		},
	}
	cmd.Flags().String("contract", "", "Contract to use to query.  Default will use the native asset to query.")
	return cmd
}

func CmdTxInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tx-input <address>",
		Aliases: []string{"input"},
		Short:   "Check inputs for a new transaction.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			addressRaw := args[0]

			addressTo, _ := cmd.Flags().GetString("to")
			contract, _ := cmd.Flags().GetString("contract")
			cli, err := xcFactory.NewClient(assetConfig(chain, contract, 0))
			if err != nil {
				return err
			}

			from := xcFactory.MustAddress(chain, addressRaw)
			to := xcFactory.MustAddress(chain, addressTo)
			input, err := cli.FetchLegacyTxInput(context.Background(), from, to)
			if err != nil {
				return fmt.Errorf("could not fetch transaction inputs: %v", err)
			}

			bz, _ := json.MarshalIndent(input, "", "  ")
			fmt.Println(string(bz))
			return nil
		},
	}
	cmd.Flags().String("contract", "", "Optional contract of token asset")
	cmd.Flags().String("to", "", "Optional destination address")
	return cmd
}

func CmdTxInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tx-info <hash>",
		Aliases: []string{"tx"},
		Short:   "Check an existing transaction on chain.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			hash := args[0]

			cli, err := xcFactory.NewClient(assetConfig(chain, "", 0))
			if err != nil {
				return err
			}

			input, err := cli.FetchTxInfo(context.Background(), xc.TxHash(hash))
			if err != nil {
				return fmt.Errorf("could not fetch tx info: %v", err)
			}

			bz, _ := json.MarshalIndent(input, "", "  ")
			fmt.Println(string(bz))
			return nil
		},
	}
	return cmd
}

func CmdTxTransfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transfer <to> <amount>",
		Aliases: []string{"tf"},
		Short:   "Create and broadcast a new transaction transferring funds. The amount should be a decimal amount.",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			to := args[0]
			amountHuman, err := xc.NewAmountHumanReadableFromStr(args[1])
			if err != nil {
				return err
			}
			contract, err := cmd.Flags().GetString("contract")
			if err != nil {
				return err
			}
			decimalsStr, err := cmd.Flags().GetString("decimals")
			if err != nil {
				return err
			}
			memo, err := cmd.Flags().GetString("memo")
			if err != nil {
				return err
			}
			timeout, err := cmd.Flags().GetDuration("timeout")
			if err != nil {
				return err
			}
			if decimalsStr == "" && contract != "" {
				return fmt.Errorf("must set --decimals if using --contract")
			}
			decimals := chain.GetDecimals()
			if contract != "" {
				parsed, err := strconv.ParseUint(decimalsStr, 10, 32)
				if err != nil {
					return fmt.Errorf("invalid decimals: %v", err)
				}
				decimals = int32(parsed)
			}

			amountBlockchain := amountHuman.ToBlockchain(decimals)

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			fromPrivateKey := xcFactory.MustPrivateKey(chain, privateKeyInput)
			signer, _ := xcFactory.NewSigner(chain)
			publicKey, err := signer.PublicKey(fromPrivateKey)
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chain)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			logrus.WithField("address", from).Info("sending from")

			cli, err := xcFactory.NewClient(assetConfig(chain, contract, decimals))
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			input, err := cli.FetchLegacyTxInput(context.Background(), from, xc.Address(to))
			if err != nil {
				return fmt.Errorf("could not fetch transfer input: %v", err)
			}

			if inputWithPublicKey, ok := input.(xc.TxInputWithPublicKey); ok {
				inputWithPublicKey.SetPublicKey(publicKey)
				logrus.WithField("public_key", hex.EncodeToString(publicKey)).Debug("added public key to transfer input")
			}
			if inputWithAmount, ok := input.(xc.TxInputWithAmount); ok {
				inputWithAmount.SetAmount(amountBlockchain)
			}
			if memo != "" {
				if txInputWithMemo, ok := input.(xc.TxInputWithMemo); ok {
					txInputWithMemo.SetMemo(memo)
				} else {
					return fmt.Errorf("cannot set memo; chain driver currently does not support memos")
				}
			}
			bz, _ := json.Marshal(input)
			logrus.WithField("input", string(bz)).Debug("transfer input")

			// create tx
			// (no network, no private key needed)
			builder, err := xcFactory.NewTxBuilder(assetConfig(chain, contract, decimals))
			if err != nil {
				return fmt.Errorf("could not load tx-builder: %v", err)
			}
			tx, err := builder.NewTransfer(from, xc.Address(to), amountBlockchain, input)
			if err != nil {
				return fmt.Errorf("could not build transfer: %v", err)
			}
			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []xc.TxSignature{}
			for _, sighash := range sighashes {
				// sign the tx sighash(es)
				signature, err := signer.Sign(fromPrivateKey, sighash)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
			}

			// complete the tx by adding its signature
			// (no network, no private key needed)
			err = tx.AddSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}

			// submit the tx, wait a bit, fetch the tx info
			// (network needed)
			err = cli.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			logrus.WithField("hash", tx.Hash()).Info("submitted tx")
			start := time.Now()
			for time.Since(start) < timeout {
				time.Sleep(5 * time.Second)
				info, err := cli.FetchTxInfo(context.Background(), tx.Hash())
				if err != nil {
					logrus.WithField("hash", tx.Hash()).WithError(err).Info("could not find tx on chain yet, trying again...")
					continue
				}
				bz, _ := json.MarshalIndent(info, "", "  ")
				fmt.Printf("%s\n", string(bz))
				return nil
			}

			return fmt.Errorf("could not find transaction that we submitted by hash %s", tx.Hash())
		},
	}
	cmd.Flags().String("contract", "", "contract address of asset to send, if applicable")
	cmd.Flags().String("decimals", "", "decimals of the token, when using --contract.")
	cmd.Flags().String("memo", "", "set a memo for the transfer.")
	cmd.Flags().Duration("timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
	return cmd
}

func CmdAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Derive an address from the PRIVATE_KEY environment variable.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			fromPrivateKey := xcFactory.MustPrivateKey(chain, privateKeyInput)
			signer, _ := xcFactory.NewSigner(chain)
			publicKey, err := signer.PublicKey(fromPrivateKey)
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chain)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			fmt.Println(from)
			return nil
		},
	}
	return cmd
}

func CmdChains() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "List information on all supported chains.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())

			cli, err := xcFactory.NewClient(assetConfig(chain, "", 0))
			if err != nil {
				return err
			}

			if xccli, ok := cli.(*crosschain.Client); ok {
				logrus.Info("listing from remote configuration")
				apiURL := fmt.Sprintf("%s/v1/chains", xccli.URL)
				res, err := xccli.ApiCallWithUrl(context.Background(), "GET", apiURL, nil)
				if err != nil {
					return err
				}
				var data any
				json.Unmarshal(res, &data)
				res, _ = json.MarshalIndent(data, "", "  ")

				fmt.Println(string(res))
			} else {
				logrus.Info("listing from local configuration")
				chains := []*xc.ChainConfig{}
				for _, asset := range xcFactory.GetAllAssets() {
					if chain, ok := asset.(*xc.ChainConfig); ok {
						chains = append(chains, chain)
					}
				}
				chainsBz, _ := json.MarshalIndent(chains, "", "  ")
				fmt.Println(string(chainsBz))
			}

			return nil
		},
	}
	return cmd
}
