package commands

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	xclienterrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdTxTransfer() *cobra.Command {
	var inclusiveFee bool
	var feePayer bool
	var dryRun bool
	var fromSecretRef string
	var feePayerSecretRef string
	var previousAttempts []string
	var txTime int64
	var addressFormat string
	var nonDeterministic bool
	var transferInputFile string

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
			addressArgs = append(addressArgs, xcaddress.OptionFormat(xc.AddressFormat(addressFormat)))
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

			privateKeyInput, err := config.GetSecret(fromSecretRef)
			if err != nil {
				return fmt.Errorf("could not get from-address secret: %v", err)
			}
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
				builder.OptionTransactionAttempts(previousAttempts),
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
				feePayerPrivateKey, err := config.GetSecret(feePayerSecretRef)
				if err != nil {
					return fmt.Errorf("could not get fee-payer secret: %v", err)
				}
				if feePayerPrivateKey == "" {
					return fmt.Errorf("fee-payer secret reference loaded an empty value")
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

			// validate to address
			err = drivers.ValidateAddress(chainConfig.Base(), xc.Address(toWalletAddress))
			if err != nil {
				return fmt.Errorf("invalid to address: %v", err)
			}

			tfArgs, err := builder.NewTransferArgs(chainConfig.Base(), from, xc.Address(toWalletAddress), amountBlockchain, tfOptions...)
			if err != nil {
				return fmt.Errorf("invalid transfer args: %v", err)
			}

			// Get input from RPC or file
			var input xc.TxInput
			if transferInputFile != "" {
				inputBz, err := os.ReadFile(transferInputFile)
				if err != nil {
					return fmt.Errorf("could not read transfer input file: %v", err)
				}
				input, err = drivers.UnmarshalTxInput(inputBz)
				if err != nil {
					return fmt.Errorf("could not unmarshal transfer input: %v", err)
				}
			} else {
				input, err = client.FetchTransferInput(context.Background(), tfArgs)
				if err != nil {
					return fmt.Errorf("could not fetch transfer input: %v", err)
				}
			}

			input, err = xcFactory.TxInputRoundtrip(input)
			if err != nil {
				return fmt.Errorf("failed tx input roundtrip: %w", err)
			}

			// set params on input that are enforced by the builder (rather than depending soley on untrusted RPC)
			timeStamp, _ := tfArgs.GetTimestamp()
			priorityMaybe, _ := tfArgs.GetPriority()
			input, err = builder.WithTxInputOptions(input, timeStamp, priorityMaybe)
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
				deductedAmount := amount.Sub(&fee)
				logrus.WithFields(logrus.Fields{
					"fee":             fee.ToHuman(chainConfig.GetDecimals()),
					"original_amount": amount.ToHuman(chainConfig.GetDecimals()),
					"new_amount":      deductedAmount.ToHuman(chainConfig.GetDecimals()),
				}).Info("deducting fee from amount")
				tfArgs.SetAmount(deductedAmount)
				tfArgs.SetInclusiveFeeSpending(true)
			}

			err = xc.CheckFeeLimit(input, chainConfig)
			if err != nil {
				return err
			}

			bz, _ := json.Marshal(input)
			logrus.WithField("input", string(bz)).Debug("transfer input")

			// By default we repeat getting .Sighashes() and .Serialize() both to test for non-determinism.
			// In Treasury we need this to be deterministic.
			var numberOfTrials = 10
			if nonDeterministic {
				numberOfTrials = 0
			}
			var sighashes []*xc.SignatureRequest
			var lastSighashes []*xc.SignatureRequest
			var lastSerializedTx []byte
			for i := range numberOfTrials {
				// create tx (no network, no private key needed)
				// use TraceLevel for checks
				tx, err := PrepareTransferForSubmit(txBuilder, tfArgs, input, signerCollection, logrus.TraceLevel)
				if err != nil {
					return fmt.Errorf("could not build transfer: %v", err)
				}

				sighashes, err = tx.Sighashes()
				if err != nil {
					return fmt.Errorf("could not create payloads to sign: %v", err)
				}

				txBytes, err := tx.Serialize()
				if err != nil {
					return fmt.Errorf("could not serialize tx: %v", err)
				}

				if i > 0 {
					// check for non-determinism
					if len(sighashes) != len(lastSighashes) {
						return fmt.Errorf("sighashes are not deterministic, differed on trial %d", i)
					}
					for j := range sighashes {
						if !bytes.Equal(sighashes[j].Payload, lastSighashes[j].Payload) {
							return fmt.Errorf("sighashes are not deterministic, differed on trial %d", i)
						}
					}

					if !bytes.Equal(txBytes, lastSerializedTx) {
						return fmt.Errorf("tx .Serialize() is not deterministic, differed on trial %d", i)
					}
				}
				lastSighashes = sighashes
				lastSerializedTx = txBytes
			}

			// recreate tx after determinism checks
			// create tx (no network, no private key needed)
			// use Info lvl for proper transaction
			tx, err := PrepareTransferForSubmit(txBuilder, tfArgs, input, signerCollection, logrus.InfoLevel)
			if err != nil {
				return fmt.Errorf("could not build transfer: %v", err)
			}

			if dryRun {
				txBytes, err := tx.Serialize()
				if err != nil {
					return fmt.Errorf("could not serialize tx: %v", err)
				}
				if utf8.Valid(txBytes) {
					fmt.Println(string(txBytes))
				} else {
					fmt.Println(hex.EncodeToString(txBytes))
				}
				return nil
			}

			// submit the tx, wait a bit, fetch the tx info (network needed)
			err = SubmitTransaction(client, tx, timeout)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			logrus.WithField("hash", tx.Hash()).Info("submitted tx")

			time.Sleep(1 * time.Second)
			logrus.Info("fetching transaction...")
			start := time.Now()

			infoArgs := []txinfo.Option{}
			infoArgs = append(infoArgs, txinfo.OptionSender(from))
			if contract != "" {
				infoArgs = append(infoArgs, txinfo.OptionContract(xc.ContractAddress(contract)))
			}
			if txTime != 0 {
				infoArgs = append(infoArgs, txinfo.OptionSignTime(txTime))
			}
			txInfoArgs := txinfo.NewArgs(tx.Hash(), infoArgs...)
			for time.Since(start) < timeout {
				info, err := client.FetchTxInfo(context.Background(), txInfoArgs)
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
	cmd.Flags().StringVar(&fromSecretRef, "from", "env:"+signer.EnvPrivateKey, "Secret reference for the from-address private key")
	cmd.Flags().StringVar(&feePayerSecretRef, "fee-payer-secret", "env:"+signer.EnvPrivateKeyFeePayer, "Secret reference for the fee-payer address private key")
	cmd.Flags().String("contract", "", "Contract address of asset to send, if applicable")
	cmd.Flags().String("decimals", "", "Decimals of the token, when using --contract.")
	cmd.Flags().String("memo", "", "Set a memo for the transfer.")
	cmd.Flags().BoolVar(&feePayer, "fee-payer", false, "Use another address to pay the fee for the transaction (uses --fee-payer-secret)")
	cmd.Flags().String("priority", "", "Apply a priority for the transaction fee ('low', 'market', 'aggressive', 'very-aggressive', or any positive decimal number)")
	cmd.Flags().Duration("timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
	cmd.Flags().BoolVar(&inclusiveFee, "inclusive-fee", false, "Include the fee in the transfer amount.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the transaction, printing it, but not submitting it.")
	cmd.Flags().StringSliceVar(&previousAttempts, "previous", []string{}, "List of transaction hashes that have been attempted and may still be in the mempool.")
	cmd.Flags().Int64Var(&txTime, "tx-time", 0, "Block time of the transaction")
	cmd.Flags().StringVar(&addressFormat, "address-format", "", "format of the address")
	cmd.Flags().BoolVar(&nonDeterministic, "non-deterministic", false, "Skip implementation checks for determinism (only important in for consensus sensitive contexts)")
	cmd.Flags().StringVar(&transferInputFile, "input", "", "File containing the transfer input.  If used, will skip fetching the input from the RPC.")
	return cmd
}

func PrepareTransferForSubmit(b builder.FullTransferBuilder, args builder.TransferArgs, input xc.TxInput, signerCollection *signer.Collection, logLevel logrus.Level) (xc.Tx, error) {
	// create tx (no network, no private key needed)
	tx, err := b.Transfer(args, input)
	if err != nil {
		return nil, fmt.Errorf("could not build transfer: %v", err)
	}

	signatures := []*xc.SignatureResponse{}

	sighashes, err := tx.Sighashes()
	if err != nil {
		return nil, fmt.Errorf("could not create payloads to sign: %v", err)
	}

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
			WithField("signature", hex.EncodeToString(signature.Signature)).Log(logLevel, "adding signature")
	}

	// complete the tx by adding its signature
	// (no network, no private key needed)
	err = tx.SetSignatures(signatures...)
	if err != nil {
		return nil, fmt.Errorf("could not add signature(s): %v", err)
	}

	if txMoreSigs, ok := tx.(xc.TxAdditionalSighashes); ok {
		for {
			additionalSighashes, err := txMoreSigs.AdditionalSighashes()
			if err != nil {
				return nil, fmt.Errorf("could not get additional sighashes: %v", err)
			}
			if len(additionalSighashes) == 0 {
				break
			}
			for _, additionalSighash := range additionalSighashes {
				log := logrus.WithField("payload", hex.EncodeToString(additionalSighash.Payload))
				signature, err := signerCollection.Sign(additionalSighash.Signer, additionalSighash.Payload)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
				log.
					WithField("address", signature.Address).
					WithField("signature", hex.EncodeToString(signature.Signature)).Log(logLevel, "adding additional signature")
			}
			err = tx.SetSignatures(signatures...)
			if err != nil {
				return nil, fmt.Errorf("could not add additional signature(s): %v", err)
			}
		}
	}

	return tx, nil
}

// Submit transaction and properly handle PreconditionFailed errors
func SubmitTransaction(client xclient.Client, tx xc.Tx, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		// submit the tx, wait a bit, fetch the tx info (network needed)
		err := client.SubmitTx(context.Background(), tx)
		if err != nil && strings.Contains(err.Error(), string(xclienterrors.FailedPrecondition)) {
			time.Sleep(time.Second * 3)
			continue
		}

		if err != nil {
			return fmt.Errorf("could not broadcast: %v", err)
		}

		return nil
	}

	return nil
}
