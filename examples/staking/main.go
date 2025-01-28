package main

import (
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/examples/staking/kiln"
	"github.com/cordialsys/crosschain/examples/staking/solana"
	"github.com/cordialsys/crosschain/examples/staking/twinstake"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdStaking() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "staking",
		Aliases:      []string{"ts"},
		Short:        "Manually interact with staking on blockchains",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			args, err := setup.RpcArgsFromCmd(cmd)
			if err != nil {
				return err
			}
			setup.ConfigureLogger(args)
			xcFactory, err := setup.LoadFactory(args)
			if err != nil {
				return err
			}
			chainConfig, err := setup.LoadChain(xcFactory, args.Chain)
			if err != nil {
				return err
			}
			setup.OverrideChainSettings(chainConfig, args)

			stakingArgs, err := setup.StakingArgsFromCmd(cmd)
			if err != nil {
				return err
			}

			var stakingCfg *services.ServicesConfig
			if stakingArgs.ConfigPath != "" {
				stakingCfg, err = services.LoadConfigFromFile(xcFactory.GetNetworkSelector(), stakingArgs.ConfigPath)
			} else {
				stakingCfg, err = services.LoadConfig(xcFactory.GetNetworkSelector())
			}
			if err != nil {
				return err
			}
			ctx := setup.CreateContext(xcFactory, chainConfig)
			ctx = setup.WrapStakingArgs(ctx, stakingArgs)
			ctx = setup.WrapStakingConfig(ctx, stakingCfg)
			clientUrl, _ := chainConfig.ClientURL()

			logrus.WithFields(logrus.Fields{
				"rpc":     clientUrl,
				"network": chainConfig.CrosschainClient.Network,
				"chain":   chainConfig.Chain,
			}).Info("chain")
			cmd.SetContext(ctx)
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {},
	}
	setup.AddRpcArgs(cmd)
	setup.AddStakingArgs(cmd)

	cmd.AddCommand(twinstake.CmdTwinstake())
	cmd.AddCommand(kiln.CmdKiln())
	cmd.AddCommand(solana.CmdSolana())

	return cmd
}

func main() {
	rootCmd := CmdStaking()
	_ = rootCmd.Execute()
}
