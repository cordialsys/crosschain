package kiln

import (
	"github.com/spf13/cobra"
)

func CmdKiln() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kiln",
		Short:        "Using kiln provider",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.AddCommand(CmdKilnTest())
	cmd.AddCommand(CmdGetStake())
	cmd.AddCommand(CmdStake())
	cmd.AddCommand(CmdUnstake())
	cmd.AddCommand(CmdConfig())
	return cmd
}
