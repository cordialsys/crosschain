package canton

import "github.com/spf13/cobra"

func CmdCanton() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "canton",
		Short:        "Canton-specific inspection commands",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
	cmd.AddCommand(CmdPreapproval())
	cmd.AddCommand(CmdTraffic())
	return cmd
}
