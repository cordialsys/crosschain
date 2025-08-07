package commands

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdTxInfo() *cobra.Command {
	var contract string
	var sender string
	var tx_time uint64
	cmd := &cobra.Command{
		Use:     "tx-info <hash>",
		Aliases: []string{"tx"},
		Short:   "Check an existing transaction on chain.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			hash := args[0]

			options := []txinfo.Option{}
			contract, err := cmd.Flags().GetString("contract")
			if err != nil {
				return err
			}
			if contract != "" {
				options = append(options, txinfo.OptionContract(xc.ContractAddress(contract)))
			}
			if sender != "" {
				options = append(options, txinfo.OptionSender(sender))
			}
			if tx_time != 0 {
				options = append(options, txinfo.OptionTxTime(tx_time))
			}
			txInfoArgs := txinfo.NewArgs(xc.TxHash(hash), options...)

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
	cmd.Flags().StringVar(&sender, "sender", "", "Public key of transaction sender")
	cmd.Flags().Uint64Var(&tx_time, "tx_time", 0, "Time of the transaction")
	return cmd
}
