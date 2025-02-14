package commands

import (
	"context"
	"fmt"
	"math/big"

	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdRpcBlock() *cobra.Command {
	var contract string
	cmd := &cobra.Command{
		Use:   "block [height]",
		Short: "Fetch the latest or other specific block",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			var blockArgs = xcclient.LatestHeight()
			if len(args) > 0 {
				h := big.NewInt(0)
				_, ok := h.SetString(args[0], 0)
				if !ok {
					return fmt.Errorf("invalid height, should be numeric: %s", args[0])
				}
				blockArgs = xcclient.AtHeight(h.Uint64())
			}

			client, err := xcFactory.NewClient(assetConfig(chainConfig, contract, 0))
			if err != nil {
				return err
			}

			block, err := client.FetchBlock(context.Background(), blockArgs)
			if err != nil {
				return fmt.Errorf("could not fetch block: %v", err)
			}
			fmt.Println(asJson(block))
			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	return cmd
}
