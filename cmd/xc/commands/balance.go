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
	var format string
	cmd := &cobra.Command{
		Use:   "balance [address]",
		Short: "Check balance of an asset.  Reported as big integer, not accounting for any decimals.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			address, err := inputAddressOrDerived(xcFactory, chainConfig, args, privateKeyRef, format)
			if err != nil {
				return err
			}

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			options := []client.GetBalanceOption{}
			if contract != "" {
				options = append(options, client.BalanceOptionContract(xc.ContractAddress(contract)))
			}
			balanceArgs := client.NewBalanceArgs(address, options...)

			balance, err := rpcClient.FetchBalance(context.Background(), balanceArgs)
			if err != nil {
				return fmt.Errorf("could not fetch balance for address %s: %v", address, err)
			}
			if decimal {
				// For native assets, pass empty string to FetchDecimals
				// For tokens, pass the contract address
				contractForDecimals := xc.ContractAddress(contract)
				decimals, err := rpcClient.FetchDecimals(context.Background(), contractForDecimals)
				if err != nil {
					return fmt.Errorf("could not fetch decimals: %v", err)
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
	cmd.Flags().StringVar(&format, "format", "", "Optional address format for chains that use multiple address formats")
	return cmd
}
