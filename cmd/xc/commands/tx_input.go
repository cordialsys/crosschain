package commands

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/signer"
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
	cmd := &cobra.Command{
		Use:     "tx-input [address]",
		Aliases: []string{"input"},
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
				privateKeyInput := signer.ReadPrivateKeyEnv()
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
	return cmd
}
