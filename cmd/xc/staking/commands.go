package staking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	var dryRun, offline bool
	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			offline = dryRun || offline

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			from, signer, err := LoadPrivateKey(xcFactory, chain)
			if err != nil {
				return err
			}
			stakingBuilder, err := xcFactory.NewStakingTxBuilder(chain.Base())
			if err != nil {
				return err
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakingArgs, err := builder.NewStakeArgs(chain.Chain, from, amount, moreArgs.BuilderOptionsWith(signer.MustPublicKey())...)
			if err != nil {
				return err
			}

			stakingInput, err := client.FetchStakingInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}
			inputBz, _ := json.MarshalIndent(stakingInput, "", "  ")
			logrus.WithField("input", string(inputBz)).Debug("input")

			tx, err := stakingBuilder.Stake(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			_, err = SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "do not broadcast the signed transaction")
	cmd.Flags().BoolVar(&offline, "offline", false, "do not broadcast the signed transaction")
	cmd.Flags().Lookup("offline").Hidden = true
	return cmd
}

func CmdUnstake() *cobra.Command {
	var dryRun, offline bool
	cmd := &cobra.Command{
		Use:   "unstake",
		Short: "Unstake an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			offline = dryRun || offline

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			from, signer, err := LoadPrivateKey(xcFactory, chain)
			if err != nil {
				return err
			}

			stakingBuilder, err := xcFactory.NewStakingTxBuilder(chain.Base())
			if err != nil {
				return err
			}

			stakingClient, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakingArgs, err := builder.NewStakeArgs(chain.Chain, from, amount, moreArgs.BuilderOptionsWith(signer.MustPublicKey())...)
			if err != nil {
				return err
			}

			stakingInput, err := stakingClient.FetchUnstakingInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}

			tx, err := stakingBuilder.Unstake(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			hash, err := SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
			if err != nil {
				return err
			}

			txInfo, err := WaitForTx(xcFactory, chain, hash, 1)
			if err != nil {
				return err
			}
			jsonprint(txInfo)
			if manualClient, ok := stakingClient.(client.ManualUnstakingClient); ok {
				logrus.Debug("chain does not support unstaking; using 3rd-party manual unstaking client")
				for _, unstake := range txInfo.Unstakes {
					if strings.EqualFold(string(from), unstake.Address) {
						err = manualClient.CompleteManualUnstaking(context.Background(), unstake)
						if err != nil {
							logrus.WithError(err).Warn("could not request exit from staking provider")
						}
					} else {
						logrus.WithField("address", unstake.Address).Debug("not our address")
					}
				}
			} else {
				logrus.Debug("chain supports unstaking; no need for manual completion")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "do not broadcast the signed transaction")
	cmd.Flags().BoolVar(&offline, "offline", false, "do not broadcast the signed transaction")
	cmd.Flags().Lookup("offline").Hidden = true
	return cmd
}

func CmdWithdraw() *cobra.Command {
	var dryRun, offline bool
	cmd := &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw from a stake account.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			offline = dryRun || offline

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)
			from, signer, err := LoadPrivateKey(xcFactory, chain)
			if err != nil {
				return err
			}

			stakingBuilder, err := xcFactory.NewStakingTxBuilder(chain.Base())
			if err != nil {
				return err
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, moreArgs.Provider)
			if err != nil {
				return err
			}

			stakingArgs, err := builder.NewStakeArgs(chain.Chain, from, amount, moreArgs.BuilderOptionsWith(signer.MustPublicKey())...)
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

			_, err = SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "do not broadcast the signed transaction")
	cmd.Flags().BoolVar(&offline, "offline", false, "do not broadcast the signed transaction")
	cmd.Flags().Lookup("offline").Hidden = true
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
