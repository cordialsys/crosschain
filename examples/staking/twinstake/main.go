package twinstake

import (
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdTwinstake() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "twinstake",
		Aliases:      []string{"ts"},
		Short:        "Using twinstake provider",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
	setup.AddRpcArgs(cmd)
	setup.AddStakingArgs(cmd)

	cmd.AddCommand(CmdTwinstakeTest())

	return cmd
}
