package staking

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
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
				fromWallet, _, err := LoadPrivateKey(xcFactory, chain)
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

func CmdStake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			from, signer, err := LoadPrivateKey(xcFactory, chain)
			if err != nil {
				return err
			}
			stakingBuilder, err := xcFactory.NewStakingTxBuilder(chain)
			if err != nil {
				return err
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakingArgs, err := builder.NewStakeArgs(from, amount, moreArgs.ToBuilderOptions()...)
			if err != nil {
				return err
			}

			stakingInput, err := client.FetchStakingInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}

			tx, err := stakingBuilder.Stake(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			return SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
		},
	}
	cmd.Flags().Bool("offline", false, "do not broadcast the signed transaction")
	return cmd
}

func CmdUnstake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unstake",
		Short: "Unstake an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			from, signer, err := LoadPrivateKey(xcFactory, chain)
			if err != nil {
				return err
			}

			stakingBuilder, err := xcFactory.NewStakingTxBuilder(chain)
			if err != nil {
				return err
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakingArgs, err := builder.NewStakeArgs(from, amount, moreArgs.ToBuilderOptions()...)
			if err != nil {
				return err
			}

			stakingInput, err := client.FetchUnstakingInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}

			tx, err := stakingBuilder.Unstake(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			return SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
		},
	}
	cmd.Flags().Bool("offline", false, "do not broadcast the signed transaction")
	return cmd
}

func CmdWithdraw() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw from a stake account.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)
			from, signer, err := LoadPrivateKey(xcFactory, chain)
			if err != nil {
				return err
			}

			stakingBuilder, err := xcFactory.NewStakingTxBuilder(chain)
			if err != nil {
				return err
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakingArgs, err := builder.NewStakeArgs(from, amount, moreArgs.ToBuilderOptions()...)
			if err != nil {
				return err
			}

			stakingInput, err := client.FetchWithdrawInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}

			tx, err := stakingBuilder.Withdraw(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			return SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
		},
	}
	cmd.Flags().Bool("offline", false, "do not broadcast the signed transaction")
	return cmd
}

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

			stakeArgs, err := builder.NewStakeArgs(xc.Address(from), amount, moreArgs.ToBuilderOptions()...)
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

			stakeArgs, err := builder.NewStakeArgs(xc.Address(from), amount, moreArgs.ToBuilderOptions()...)
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

			stakeArgs, err := builder.NewStakeArgs(xc.Address(from), amount, stakingArgs.ToBuilderOptions()...)
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
