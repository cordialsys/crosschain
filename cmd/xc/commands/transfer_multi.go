package commands

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdTxMultiTransfer() *cobra.Command {
	var inclusiveFee bool
	var feePayer bool
	var dryRun bool

	var timeout time.Duration
	var priorityStr string
	// var fromSecretRef string

	var fromSecretRefs []string
	var toAddresses []string
	var feePayerSecretRef string
	var algorithms []string

	var amountsRaw []string
	var contracts []string
	var decimals []int

	doFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringSliceVar(&fromSecretRefs, "from", []string{}, "Secret references for the from-address private keys")
		cmd.Flags().StringSliceVar(&toAddresses, "to", []string{}, "Addresses to send funds to")
		cmd.Flags().StringSliceVar(&amountsRaw, "amount", []string{}, "Amounts to send to each address, respectively")

		cmd.Flags().StringSliceVar(&contracts, "contract", []string{}, "Contract addresses of assets to send, if applicable")
		cmd.Flags().IntSliceVar(&decimals, "decimals", []int{}, "Decimals of the tokens, when using --contract.")

		cmd.Flags().StringSliceVar(&algorithms, "algorithm", []string{}, "Algorithms to use for each address, if applicable.  Can use empty string for default algorithm.")

		cmd.Flags().StringVar(&feePayerSecretRef, "fee-payer-secret", "env:"+signer.EnvPrivateKeyFeePayer, "Secret reference for the fee-payer address private key")

		cmd.Flags().BoolVar(&feePayer, "fee-payer", false, "Use another address to pay the fee for the transaction (uses --fee-payer-secret)")
		cmd.Flags().StringVar(&priorityStr, "priority", "", "Apply a priority for the transaction fee ('low', 'market', 'aggressive', 'very-aggressive', or any positive decimal number)")
		cmd.Flags().DurationVar(&timeout, "timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
		cmd.Flags().BoolVar(&inclusiveFee, "inclusive-fee", false, "Include the fee in the transfer amount.")
		cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the transaction, printing it, but not submitting it.")
	}

	cmd := &cobra.Command{
		Use:     "multi-transfer",
		Aliases: []string{"multi"},
		Short:   "Create and broadcast a new transaction with multiple transfers. ",
		Long: "On UTXO chains like Bitcoin, the first from-address will receive the change." +
			"Otherwise, the order of the from-addresses does not matter, and the number of from-addresses does not need to match the number of to-addresses." +
			"\n" +
			"Whereas on account-based chains like Solana there can only be one from address." +
			"Use empty \"\" value or chain-id for contract or decimals if sending native asset.",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			if len(contracts) != len(decimals) {
				return fmt.Errorf("must set --decimals if using --contract")
			}

			if len(fromSecretRefs) == 0 {
				return fmt.Errorf("must set at least one --from")
			}
			if len(toAddresses) == 0 {
				return fmt.Errorf("must set at least one --to")
			}
			if len(amountsRaw) != len(toAddresses) {
				return fmt.Errorf("must set at one --amount for each --to")
			}

			if len(algorithms) != 0 && len(algorithms) != len(fromSecretRefs) {
				return fmt.Errorf("must set one --algorithm for each --from (omit or use empty string for default)")
			}
			if len(algorithms) == 0 {
				algorithms = make([]string, len(fromSecretRefs))
			}

			// technically you could have vary number of contracts for both from and to (e.g. on Sui),
			// but we're going to ignore that until it's actually fully implemented
			if len(contracts) != 0 && len(contracts) != len(toAddresses) {
				return fmt.Errorf("must set one --contract for each --to, or omit --contract")
			}
			if len(contracts) == 0 {
				contracts = make([]string, len(toAddresses))
			}

			// convert balances to balances
			balances := make([]xc.AmountBlockchain, len(amountsRaw))
			for i, amountRaw := range amountsRaw {
				contract := contracts[i]
				decimalsForAmount := int(chainConfig.GetDecimals())
				if contract != "" && contract != string(chainConfig.Chain) {
					decimalsForAmount = decimals[i]
				}
				amountHuman, err := xc.NewAmountHumanReadableFromStr(amountRaw)
				if err != nil {
					return fmt.Errorf("invalid amount '%s' at position %d: %v", amountRaw, i, err)
				}
				amount := amountHuman.ToBlockchain(int32(decimalsForAmount))
				balances[i] = amount
			}

			signers := signer.NewCollection()
			senders := make([]*builder.Sender, len(fromSecretRefs))
			for i, fromSecretRef := range fromSecretRefs {
				fromSecret, err := config.GetSecret(fromSecretRef)
				if err != nil {
					return fmt.Errorf("could not get from-address secret at %d position: %v", i, err)
				}
				if fromSecret == "" {
					return fmt.Errorf("from address at position %d loaded a blank value for secret", i)
				}
				algorithm := algorithms[i]
				addressArgs := []xcaddress.AddressOption{}
				if algorithm != "" {
					addressArgs = append(addressArgs, xcaddress.OptionAlgorithm(xc.SignatureType(algorithm)))
				}

				signer, err := xcFactory.NewSigner(chainConfig.Base(), fromSecret, addressArgs...)
				if err != nil {
					return fmt.Errorf("could not create signer for from-address at position %d: %v", i, err)
				}
				// signers.AddAuxSigner(signer, )
				addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
				if err != nil {
					return fmt.Errorf("could not create address builder: %v", err)
				}
				publicKey, err := signer.PublicKey()
				if err != nil {
					return fmt.Errorf("could not create public key for from-address at position %d: %v", i, err)
				}

				address, err := addressBuilder.GetAddressFromPublicKey(publicKey)
				if err != nil {
					return fmt.Errorf("could not derive address for from-address at position %d: %v", i, err)
				}
				logrus.WithField("address", address).Info("sending from")

				signers.AddAuxSigner(signer, address)
				senders[i], err = builder.NewSender(address, publicKey)
				if err != nil {
					return fmt.Errorf("could not create spender for from-address at position %d: %v", i, err)
				}
			}
			receivers := make([]*builder.Receiver, len(toAddresses))
			for i := range toAddresses {
				toAddress := xc.Address(toAddresses[i])
				amount := balances[i]
				contract := xc.ContractAddress(contracts[i])

				options := []builder.BuilderOption{}
				if contract != "" && contract != xc.ContractAddress(chainConfig.Chain) {
					decimals := decimals[i]
					options = append(options, builder.OptionContractAddress(contract))
					options = append(options, builder.OptionContractDecimals(decimals))
				}
				var err error
				receivers[i], err = builder.NewReceiver(toAddress, amount, options...)
				if err != nil {
					return fmt.Errorf("could not create receiver for to-address '%s' at position %d: %v", toAddresses[i], i, err)
				}
				logrus.WithField("address", toAddress).WithField("amount", amount).Info("sending to")
			}
			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			tfOptions := []builder.BuilderOption{
				builder.OptionTimestamp(time.Now().Unix()),
			}

			txBuilder, err := xcFactory.NewTxBuilder(chainConfig.GetChain().Base())
			if err != nil {
				return fmt.Errorf("could not load tx-builder: %v", err)
			}
			multiTxBuilder, ok := txBuilder.(builder.MultiTransfer)
			if !ok {
				return fmt.Errorf("multi-transfer on chain %s is not implemented", chainConfig.Chain)
			}
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
				addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base())
				if err != nil {
					return fmt.Errorf("could not create address builder: %v", err)
				}
				feePayerAddress, err := addressBuilder.GetAddressFromPublicKey(feePayerPublicKey)
				if err != nil {
					return fmt.Errorf("could not derive fee-payer address: %v", err)
				}
				logrus.WithField("fee-payer", feePayerAddress).Info("using fee-payer")
				tfOptions = append(tfOptions, builder.OptionFeePayer(feePayerAddress, feePayerPublicKey))
				signers.AddAuxSigner(feePayerSigner, feePayerAddress)
			}
			if priorityStr != "" {
				priority, err := xc.NewPriority(priorityStr)
				if err != nil {
					return fmt.Errorf("invalid priority: %v", err)
				}
				tfOptions = append(tfOptions, builder.OptionPriority(priority))
			}
			tfArgs, err := builder.NewMultiTransferArgs(chainConfig.Chain, senders, receivers, tfOptions...)
			if err != nil {
				return fmt.Errorf("invalid multi-transfer args: %v", err)
			}

			multiClient, ok := client.(xcclient.MultiTransferClient)
			if !ok {
				return fmt.Errorf("multi-transfer fetch-input is not supported on chain %s", chainConfig.Chain)
			}

			// Get input from RPC
			input, err := multiClient.FetchMultiTransferInput(context.Background(), *tfArgs)
			if err != nil {
				return fmt.Errorf("could not fetch multi-transfer input: %v", err)
			}

			if inclusiveFee {
				fee, feeAssetId := input.GetFeeLimit()

				err = tfArgs.DeductFee(fee, chainConfig.Chain, feeAssetId)
				if err != nil {
					return fmt.Errorf("could not deduct fee: %v", err)
				}
			}

			err = xc.CheckFeeLimit(input, chainConfig)
			if err != nil {
				return err
			}

			bz, _ := json.Marshal(input)
			logrus.WithField("input", string(bz)).Debug("transfer input")

			// create tx (no network, no private key needed)
			tx, err := multiTxBuilder.MultiTransfer(*tfArgs, input)
			if err != nil {
				return fmt.Errorf("could not build multi-transfer: %v", err)
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
				signature, err := signers.Sign(sighash.Signer, sighash.Payload)
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
			err = tx.SetSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}
			if txMoreSigs, ok := tx.(xc.TxAdditionalSighashes); ok {
				for {
					additionalSighashes, err := txMoreSigs.AdditionalSighashes()
					if err != nil {
						return fmt.Errorf("could not get additional sighashes: %v", err)
					}
					if len(additionalSighashes) == 0 {
						break
					}
					for _, additionalSighash := range additionalSighashes {
						log := logrus.WithField("payload", hex.EncodeToString(additionalSighash.Payload))
						signature, err := signers.Sign(additionalSighash.Signer, additionalSighash.Payload)
						if err != nil {
							panic(err)
						}
						signatures = append(signatures, signature)
						log.
							WithField("address", signature.Address).
							WithField("signature", hex.EncodeToString(signature.Signature)).Info("adding additional signature")
					}
					err = tx.SetSignatures(signatures...)
					if err != nil {
						return fmt.Errorf("could not add additional signature(s): %v", err)
					}
				}
			}

			if dryRun {
				txBytes, err := tx.Serialize()
				if err != nil {
					return fmt.Errorf("could not serialize tx: %v", err)
				}
				logrus.Debugf("not submitting due to --dry-run")
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
				args := xcclient.NewTxInfoArgs(tx.Hash())
				info, err := client.FetchTxInfo(context.Background(), args)
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

	doFlags(cmd)

	return cmd
}
