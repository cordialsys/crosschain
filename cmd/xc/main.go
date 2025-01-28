package main

import (
	"fmt"

	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/cmd/xc/commands"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/cmd/xc/staking"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdXc() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "xc",
		Short:        "Manually interact with blockchains",
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
			if args.UseLocalImplementation {
				xcFactory.NoXcClients = true
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

			url, _ := chainConfig.ClientURL()

			logrus.WithFields(logrus.Fields{
				"rpc":     url,
				"network": chainConfig.CrosschainClient.Network,
				"chain":   chainConfig.Chain,
			}).Info("chain")
			cmd.SetContext(ctx)
			switch cmd.Use {
			case "chains":
				// short circuit validation for some commands manually
				return nil
			}

			if args.Rpc == "" && args.UseLocalImplementation {
				return fmt.Errorf("must pass --rpc when using --local")
			}
			return nil
		},
	}
	setup.AddRpcArgs(cmd)
	setup.AddStakingArgs(cmd)

	cmd.AddCommand(commands.CmdRpcBalance())
	cmd.AddCommand(commands.CmdDecimals())
	cmd.AddCommand(commands.CmdTxInput())
	cmd.AddCommand(commands.CmdTxInfo())
	cmd.AddCommand(commands.CmdTxTransfer())
	cmd.AddCommand(commands.CmdAddress())
	cmd.AddCommand(commands.CmdChains())
	cmd.AddCommand(commands.CmdFund())
	cmd.AddCommand(staking.CmdStaking())

	return cmd
}

func main() {
	rootCmd := CmdXc()
	_ = rootCmd.Execute()
}
