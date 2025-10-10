package staking

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdStakedBalances() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance <address>",
		Short: "Lookup staked balances.",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			from := ""
			if len(args) > 0 {
				from = args[0]
			} else {
				// try loading from private-key env
				fromWallet, _, err := LoadPrivateKey(xcFactory, chain, "")
				if err != nil {
					return fmt.Errorf("must provider an address or private key env (%v)", err)
				}
				from = string(fromWallet)
			}

			stakingClient, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakeArgs, err := client.NewStakeBalanceArgs(xc.Address(from), moreArgs.ToBalanceOptions()...)
			if err != nil {
				return err
			}

			balances, err := stakingClient.FetchStakeBalance(context.Background(), stakeArgs)
			if err != nil {
				return err
			}

			jsonprint(balances)

			return nil
		},
	}
	return cmd
}
