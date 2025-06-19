package commands

import (
	"github.com/cordialsys/crosschain/cmd/xc/commands/tools"
	"github.com/spf13/cobra"
)

func CmdTools() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tools",
		Short:        "Miscellaneous utilities",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.AddCommand(tools.CmdDebug())
	cmd.AddCommand(tools.CmdEos())

	return cmd
}
