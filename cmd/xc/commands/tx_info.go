package commands

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdTxInfo() *cobra.Command {
	var contract string
	cmd := &cobra.Command{
		Use:     "tx-info <hash>",
		Aliases: []string{"tx"},
		Short:   "Check an existing transaction on chain.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			hash := args[0]

			options := []client.GetTxInfoOption{}
			contract, err := cmd.Flags().GetString("contract")
			if err != nil {
				return err
			}
			if contract != "" {
				options = append(options, client.TxInfoOptionContract(xc.ContractAddress(contract)))
			}
			txInfoArgs := client.NewTxInfoArgs(xc.TxHash(hash), options...)

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			txInfo, err := client.FetchTxInfo(context.Background(), txInfoArgs)
			if err != nil {
				return fmt.Errorf("could not fetch tx info: %v", err)
			}

			fmt.Println(asJson(txInfo))

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "")
	return cmd
}
