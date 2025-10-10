package staking

import (
	"encoding/json"
	"fmt"

	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdStake() *cobra.Command {
	var dryRun, offline bool
	var privateKeyRefMaybe string
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
