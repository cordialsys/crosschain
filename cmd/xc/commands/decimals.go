package commands

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdDecimals() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decimals",
		Short: "Lookup the configured decimals for an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract, err := cmd.Flags().GetString("contract")
			if err != nil {
				return err
			}
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			decimals, err := client.FetchDecimals(context.Background(), xc.ContractAddress(contract))
			if err != nil {
				return fmt.Errorf("could not fetch decimals for %s: %v", contract, err)
			}

			fmt.Println(decimals)

			return nil
		},
	}
	cmd.Flags().String("contract", "", "Contract to use to query.")
	return cmd
}
