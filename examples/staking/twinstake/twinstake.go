package twinstake

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client/services/twinstake"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func jsonprint(a any) {
	bz, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(bz))
}
func CmdTwinstakeTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "twinstake",
		Aliases: []string{"ts"},
		Short:   "Using twinstake provider.",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			// rpcArgs := setup.UnwrapArgs(cmd.Context())
			_ = xcFactory
			_ = chain
			bal := xc.NewAmountBlockchainFromUint64(0)
			if moreArgs.Amount != nil {
				bal = moreArgs.Amount.ToBlockchain(chain.Decimals)
			}
			_ = bal
			// apiKey, err := stakingCfg.Kiln.ApiToken.Load()
			// if err != nil {
			// 	return err
			// }

			cli, err := twinstake.NewClient(string(chain.Chain), &stakingCfg.Twinstake)
			if err != nil {
				return err
			}
			token, err := cli.Login()
			if err != nil {
				return err
			}
			jsonprint(token)
			// stakes, err := cli.GetAllStakesByOwner("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
			// if err != nil {
			// 	return err
			// }
			// jsonprint(stakes)

			return nil
		},
	}
	return cmd
}
