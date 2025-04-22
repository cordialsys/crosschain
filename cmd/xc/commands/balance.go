package commands

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func CmdRpcBalance() *cobra.Command {
	var contract string
	var decimal bool
	var privateKeyRef string
	cmd := &cobra.Command{
		Use:   "balance [address]",
		Short: "Check balance of an asset.  Reported as big integer, not accounting for any decimals.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			address, err := inputAddressOrDerived(xcFactory, chainConfig, args, privateKeyRef)
			if err != nil {
				return err
			}

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			options := []client.GetBalanceOption{}
			if contract != "" {
				options = append(options, client.OptionContract(xc.ContractAddress(contract)))
			}
			balanceArgs := client.NewBalanceArgs(address, options...)

			balance, err := rpcClient.FetchBalance(context.Background(), balanceArgs)
			if err != nil {
				return fmt.Errorf("could not fetch balance for address %s: %v", address, err)
			}
			if decimal {
				if contract == "" {
					contract = string(chainConfig.Chain)
				}
				decimals, err := rpcClient.FetchDecimals(context.Background(), xc.ContractAddress(contract))
				if err != nil {
					return fmt.Errorf("could not fetch decimals for contract %s: %v", contract, err)
				}
				amount := balance.ToHuman(int32(decimals))
				fmt.Println(amount.String())
			} else {
				fmt.Println(balance.String())
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().BoolVar(&decimal, "decimal", false, "Report balance as a decimal.  If set, the decimals will be looked up.")
	return cmd
}
