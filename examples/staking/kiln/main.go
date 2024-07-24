package main

import (
	"github.com/cordialsys/crosschain/client/staking"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdStaking() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "staking",
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

			var stakingCfg *staking.ServicesConfig
			if stakingArgs.ConfigPath != "" {
				stakingCfg, err = staking.LoadConfigFromFile(xcFactory.GetNetworkSelector(), stakingArgs.ConfigPath)
			} else {
				stakingCfg, err = staking.LoadConfig(xcFactory.GetNetworkSelector())
			}
			if err != nil {
				return err
			}
			ctx := setup.CreateContext(xcFactory, chainConfig)
			ctx = setup.WrapStakingArgs(ctx, stakingArgs)
			ctx = setup.WrapStakingConfig(ctx, stakingCfg)

			logrus.WithFields(logrus.Fields{
				"rpc":     chainConfig.GetAllClients()[0].URL,
				"network": chainConfig.GetAllClients()[0].Network,
				"chain":   chainConfig.Chain,
			}).Info("chain")
			cmd.SetContext(ctx)
			return nil
		},
	}
	setup.AddRpcArgs(cmd)
	setup.AddStakingArgs(cmd)
	cmd.Flags().String("kiln-api", "", "Override base URL for Kiln API.")

	cmd.AddCommand(CmdKiln())
	cmd.AddCommand(CmdGetStake())
	cmd.AddCommand(CmdStake())
	cmd.AddCommand(CmdUnstake())
	cmd.AddCommand(CmdConfig())
	return cmd
}

func main() {
	rootCmd := CmdStaking()
	_ = rootCmd.Execute()
}
