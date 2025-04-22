package commands

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdTxTransfer() *cobra.Command {
	var inclusiveFee bool
	var feePayer bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "transfer <to> <amount>",
		Aliases: []string{"tf"},
		Short:   "Create and broadcast a new transaction transferring funds. The amount should be a decimal amount.",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

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

			priorityStr, err := cmd.Flags().GetString("priority")
			if err != nil {
				return err
			}

			if decimalsStr == "" && contract != "" {
				return fmt.Errorf("must set --decimals if using --contract")
			}
			algorithm, _ := cmd.Flags().GetString("algorithm")
			addressArgs := []xcaddress.AddressOption{}
			if algorithm != "" {
				addressArgs = append(addressArgs, xcaddress.OptionAlgorithm(xc.SignatureType(algorithm)))
			}

			toWalletAddress := args[0]
			transferredAmount := args[1]

			decimals := chainConfig.GetDecimals()
			if contract != "" {
				parsed, err := strconv.ParseUint(decimalsStr, 10, 32)
				if err != nil {
					return fmt.Errorf("invalid decimals: %v", err)
				}
				decimals = int32(parsed)
			}

			privateKeyInput := signer.ReadPrivateKeyEnv()
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			transferredAmountHuman, err := xc.NewAmountHumanReadableFromStr(transferredAmount)
			if err != nil {
				return err
			}
			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			amountBlockchain := transferredAmountHuman.ToBlockchain(decimals)
			tfOptions := []builder.BuilderOption{
				builder.OptionTimestamp(time.Now().Unix()),
			}

			mainSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, addressArgs...)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := mainSigner.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}
			tfOptions = append(tfOptions, builder.OptionPublicKey(publicKey))

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			signerCollection := signer.NewCollection()
			signerCollection.AddMainSigner(mainSigner, from)

			txBuilder, err := xcFactory.NewTxBuilder(chainConfig.GetChain().Base())
			if err != nil {
				return fmt.Errorf("could not load tx-builder: %v", err)
			}

			logrus.WithField("address", from).Info("sending from")
			if feePayer {
				_, ok := txBuilder.(builder.BuilderSupportsFeePayer)
				if !ok {
					return fmt.Errorf("support for fee payer on chain %s is not implemented", chainConfig.Chain)
				}
				feePayerPrivateKey := signer.ReadPrivateKeyFeePayerEnv()
				if feePayerPrivateKey == "" {
					return fmt.Errorf("must set env %s", signer.EnvPrivateKeyFeePayer)
				}
				feePayerSigner, err := xcFactory.NewSigner(chainConfig.Base(), feePayerPrivateKey)
				if err != nil {
					return fmt.Errorf("could not import fee-payer private key: %v", err)
				}
				feePayerPublicKey, err := feePayerSigner.PublicKey()
				if err != nil {
					return fmt.Errorf("could not create fee-payer public key: %v", err)
				}
				feePayerAddress, err := addressBuilder.GetAddressFromPublicKey(feePayerPublicKey)
				if err != nil {
					return fmt.Errorf("could not derive fee-payer address: %v", err)
				}
				logrus.WithField("fee-payer", feePayerAddress).Info("using fee-payer")
				tfOptions = append(tfOptions, builder.OptionFeePayer(feePayerAddress, feePayerPublicKey))
				signerCollection.AddAuxSigner(feePayerSigner, feePayerAddress)
			}

			if memo != "" {
				tfOptions = append(tfOptions, builder.OptionMemo(memo))
			}
			if contract != "" {
				tfOptions = append(tfOptions, builder.OptionContractAddress(xc.ContractAddress(contract)))
			}
			if decimalsStr != "" {
				tfOptions = append(tfOptions, builder.OptionContractDecimals(int(decimals)))
			}

			if priorityStr != "" {
				priority, err := xc.NewPriority(priorityStr)
				if err != nil {
					return fmt.Errorf("invalid priority: %v", err)
				}
				tfOptions = append(tfOptions, builder.OptionPriority(priority))
			}

			tfArgs, err := builder.NewTransferArgs(from, xc.Address(toWalletAddress), amountBlockchain, tfOptions...)
			if err != nil {
				return fmt.Errorf("invalid transfer args: %v", err)
			}

			// Get input from RPC
			input, err := client.FetchTransferInput(context.Background(), tfArgs)
			if err != nil {
				return fmt.Errorf("could not fetch transfer input: %v", err)
			}

			// set params on input that are enforced by the builder (rather than depending soley on untrusted RPC)
			input, err = builder.WithTxInputOptions(input, tfArgs.GetAmount(), &tfArgs)
			if err != nil {
				return fmt.Errorf("could not apply trusted options to tx-input: %v", err)
			}

			if inclusiveFee {
				fee, feeAssetId := input.GetFeeLimit()
				if contract != "" && feeAssetId != xc.ContractAddress(contract) {
					return fmt.Errorf("cannot include fee of asset %s in transfer of asset %s", feeAssetId, contract)
				}
				if contract == "" {
					if feeAssetId == "" {
						feeAssetId = xc.ContractAddress(chainConfig.Chain)
					}
					if feeAssetId != xc.ContractAddress(chainConfig.Chain) {
						return fmt.Errorf("cannot include fee of asset %s in transfer of asset %s", feeAssetId, chainConfig.Chain)
					}
				}
				amount := tfArgs.GetAmount()
				tfArgs.SetAmount(amount.Sub(&fee))
			}

			err = xc.CheckFeeLimit(input, chainConfig)
			if err != nil {
				return err
			}

			bz, _ := json.Marshal(input)
			logrus.WithField("input", string(bz)).Debug("transfer input")

			// create tx (no network, no private key needed)
			tx, err := txBuilder.Transfer(tfArgs, input)
			if err != nil {
				return fmt.Errorf("could not build transfer: %v", err)
			}

			// serialize tx for signing
			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []*xc.SignatureResponse{}
			for _, sighash := range sighashes {
				log := logrus.WithField("payload", hex.EncodeToString(sighash.Payload))
				if len(sighash.Payload) == 0 {
					panic("requested to sign empty payload")
				}
				// sign the tx sighash(es)
				signature, err := signerCollection.Sign(sighash.Signer, sighash.Payload)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
				log.
					WithField("address", signature.Address).
					WithField("signature", hex.EncodeToString(signature.Signature)).Info("adding signature")
			}

			// complete the tx by adding its signature
			// (no network, no private key needed)
			err = tx.AddSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}
			if dryRun {
				txBytes, err := tx.Serialize()
				if err != nil {
					return fmt.Errorf("could not serialize tx: %v", err)
				}
				fmt.Println(hex.EncodeToString(txBytes))
				return nil
			}

			// submit the tx, wait a bit, fetch the tx info (network needed)
			err = client.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			logrus.WithField("hash", tx.Hash()).Info("submitted tx")

			time.Sleep(1 * time.Second)
			logrus.Info("fetching transaction...")
			start := time.Now()
			for time.Since(start) < timeout {
				info, err := client.FetchTxInfo(context.Background(), tx.Hash())
				if err != nil {
					logrus.WithField("hash", tx.Hash()).WithError(err).Info("could not find tx on chain yet, trying again in 3s...")
					time.Sleep(3 * time.Second)
					continue
				}

				if info.Confirmations < 1 {
					if logrus.GetLevel() >= logrus.DebugLevel {
						fmt.Fprintln(os.Stderr, asJson(info))
					}
					logrus.Info("waiting for confirmation...")
					time.Sleep(3 * time.Second)
					continue
				} else {
					fmt.Println(asJson(info))
					return nil
				}
			}

			return fmt.Errorf("could not find transaction that we submitted by hash %s", tx.Hash())
		},
	}
	cmd.Flags().String("contract", "", "Contract address of asset to send, if applicable")
	cmd.Flags().String("decimals", "", "Decimals of the token, when using --contract.")
	cmd.Flags().String("memo", "", "Set a memo for the transfer.")
	cmd.Flags().BoolVar(&feePayer, "fee-payer", false, "Use another address to pay the fee for the transaction (must set env PRIVATE_KEY_FEE_PAYER)")
	cmd.Flags().String("priority", "", "Apply a priority for the transaction fee ('low', 'market', 'aggressive', 'very-aggressive', or any positive decimal number)")
	cmd.Flags().Duration("timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
	cmd.Flags().BoolVar(&inclusiveFee, "inclusive-fee", false, "Include the fee in the transfer amount.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the transaction, printing it, but not submitting it.")
	return cmd
}
