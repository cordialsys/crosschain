package tools

import (
	"github.com/cordialsys/crosschain/cmd/xc/commands/tools/eostools"
	"github.com/spf13/cobra"
)

func CmdEos() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "eos",
		Short:        "Utilities for EOS chain",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.AddCommand(eostools.CmdTxTransferEOS())
	cmd.AddCommand(eostools.CmdTxCreateAccount())
	cmd.AddCommand(eostools.CmdTxStake())
	cmd.AddCommand(eostools.CmdTxBuyRam())

	return cmd
}
