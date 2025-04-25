package commands

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdTxInput() *cobra.Command {
	var addressTo string
	var amount string
	var contract string
	var publicKeyHex string
	var memo string
	var decimals int
	var privateKeyRef string
	var feePayerSecretRef string
	var feePayer bool
	cmd := &cobra.Command{
		Use:     "tx-input [address]",
		Aliases: []string{"input", "transfer-input"},
		Short:   "Check inputs for a new transaction.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addressTo, _ := cmd.Flags().GetString("to")
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			fromAddress, err := inputAddressOrDerived(xcFactory, chainConfig, args, privateKeyRef)
			if err != nil {
				return err
			}
			if contract != "" && decimals == -1 {
				return fmt.Errorf("must set --decimals if using --contract")
			}

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}

			tfOptions := []builder.BuilderOption{}
			if contract != "" {
				tfOptions = append(tfOptions, builder.OptionContractAddress(xc.ContractAddress(contract)))
				tfOptions = append(tfOptions, builder.OptionContractDecimals(decimals))
			}
			if publicKeyHex != "" {
				publicKey, err := hex.DecodeString(strings.TrimPrefix(publicKeyHex, "0x"))
				if err != nil {
					return fmt.Errorf("could not decode public key: %v", err)
				}
				tfOptions = append(tfOptions, builder.OptionPublicKey(publicKey))
			} else {
				privateKeyInput, err := config.GetSecret(privateKeyRef)
				if err != nil {
					return fmt.Errorf("could not get private key: %v", err)
				}
				if privateKeyInput != "" {
					signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
					if err != nil {
						return fmt.Errorf("could not import private key: %v", err)
					}
					publicKey, err := signer.PublicKey()
					if err != nil {
						return fmt.Errorf("could not create public key: %v", err)
					}
					tfOptions = append(tfOptions, builder.OptionPublicKey(publicKey))
				}
			}
			if memo != "" {
				tfOptions = append(tfOptions, builder.OptionMemo(memo))
			}
			addressArgs := []xcaddress.AddressOption{}
			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			if feePayer {
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
			}

			// default to smallest possible amount
			amountBlockchain := xc.NewAmountBlockchainFromUint64(1)
			if amount != "" {
				humanAmount, err := xc.NewAmountHumanReadableFromStr(amount)
				if err != nil {
					return fmt.Errorf("could parse amount: %v", err)
				}
				if decimals >= 0 {
					amountBlockchain = humanAmount.ToBlockchain(int32(decimals))
				} else {
					amountBlockchain = humanAmount.ToBlockchain(int32(chainConfig.GetDecimals()))
				}
			}

			tfArgs, err := builder.NewTransferArgs(
				fromAddress,
				xc.Address(addressTo),
				amountBlockchain,
				tfOptions...,
			)
			if err != nil {
				return fmt.Errorf("could not create transfer args: %v", err)
			}

			input, err := client.FetchTransferInput(context.Background(), tfArgs)
			if err != nil {
				return fmt.Errorf("could not fetch transaction input: %v", err)
			}

			fmt.Println(asJson(input))

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().IntVar(&decimals, "decimals", -1, "Optional decimals of the token asset")
	cmd.Flags().StringVar(&addressTo, "to", "", "Optional destination address")
	cmd.Flags().StringVar(&amount, "amount", "", "human amount to transfer")
	cmd.Flags().StringVar(&memo, "memo", "", "Optional memo for the transaction")
	cmd.Flags().StringVar(&publicKeyHex, "public-key", "",
		fmt.Sprintf("Public key in hex of the sender address (will use %s if set)", signer.EnvPrivateKey),
	)
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")

	cmd.Flags().BoolVar(&feePayer, "fee-payer", false, "Use another address to pay the fee for the transaction (uses --fee-payer-secret)")
	cmd.Flags().StringVar(&feePayerSecretRef, "fee-payer-secret", "env:"+signer.EnvPrivateKeyFeePayer, "Secret reference for the fee-payer address private key")

	return cmd
}
