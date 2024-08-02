package staking

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func jsonprint(a any) {
	bz, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(bz))
}

func CmdStaking() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "staking",
		Short:        "Staking commands",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.AddCommand(CmdStake())
	cmd.AddCommand(CmdUnstake())
	cmd.AddCommand(CmdWithdraw())
	cmd.AddCommand(CmdStakedBalances())
	cmd.AddCommand(CmdFetchStakeInput())
	cmd.AddCommand(CmdFetchUnStakeInput())
	cmd.AddCommand(CmdFetchWithdrawInput())
	return cmd
}
