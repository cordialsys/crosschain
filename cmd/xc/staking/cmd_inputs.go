package staking

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdFetchStakeInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stake-input <address>",
		Short: "Fetch inputs for a new staking transaction.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())

			from := args[0]
			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakeArgs, err := builder.NewStakeArgs(chain.Chain, xc.Address(from), amount, moreArgs.ToBuilderOptions()...)
			if err != nil {
				return err
			}

			input, err := client.FetchStakingInput(context.Background(), stakeArgs)
			if err != nil {
				return err
			}

			jsonprint(input)

			return nil
		},
	}
	return cmd
}

func CmdFetchUnstakeInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unstake-input <address>",
		Short: "Fetch inputs for a new unstake transaction.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())

			from := args[0]
			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakeArgs, err := builder.NewStakeArgs(chain.Chain, xc.Address(from), amount, moreArgs.ToBuilderOptions()...)
			if err != nil {
				return err
			}

			input, err := client.FetchUnstakingInput(context.Background(), stakeArgs)
			if err != nil {
				return err
			}

			jsonprint(input)

			return nil
		},
	}
	return cmd
}

func CmdFetchWithdrawInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw-input <address>",
		Short: "Fetch inputs for a new withdraw transaction.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			stakingArgs := setup.UnwrapStakingArgs(cmd.Context())
			servicesCfg := setup.UnwrapStakingConfig(cmd.Context())

			from := args[0]
			amountHuman := stakingArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			client, err := xcFactory.NewStakingClient(servicesCfg, chain, stakingArgs.Provider)
			if err != nil {
				return err
			}

			stakeArgs, err := builder.NewStakeArgs(chain.Chain, xc.Address(from), amount, stakingArgs.ToBuilderOptions()...)
			if err != nil {
				return err
			}

			input, err := client.FetchWithdrawInput(context.Background(), stakeArgs)
			if err != nil {
				return err
			}

			jsonprint(input)

			return nil
		},
	}
	return cmd
}
