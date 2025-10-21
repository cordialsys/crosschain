package staking

import (
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdWithdraw() *cobra.Command {
	var dryRun, offline bool
	var privateKeyRefMaybe string
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

			from, signer, err := LoadPrivateKey(xcFactory, chain, privateKeyRefMaybe)
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

			opts := moreArgs.BuilderOptionsWith(signer.MustPublicKey())
			amountHuman := moreArgs.Amount
			if amountHuman != nil {
				amount := amountHuman.ToBlockchain(chain.Decimals)
				opts = append(opts, builder.OptionStakeAmount(amount))
			}
			stakingArgs, err := builder.NewStakeArgs(chain.Chain, from, opts...)
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

			hash, err := SignAndMaybeBroadcast(xcFactory, chain, signer, tx, !offline)
			if err != nil {
				return err
			}

			txInfo, err := WaitForTx(xcFactory, chain, hash, 1)
			if err != nil {
				return err
			}
			jsonprint(txInfo)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "do not broadcast the signed transaction")
	cmd.Flags().BoolVar(&offline, "offline", false, "do not broadcast the signed transaction")
	cmd.Flags().Lookup("offline").Hidden = true
	cmd.Flags().StringVar(&privateKeyRefMaybe, "from", "", "Secret reference to use for the address")
	return cmd
}
