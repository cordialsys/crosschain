package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config/constants"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type RpcContextKey string

const ContextXc RpcContextKey = "xc"
const ContextChain RpcContextKey = "chain"

func wrapXc(ctx context.Context, xcFactory *factory.Factory) context.Context {
	ctx = context.WithValue(ctx, ContextXc, xcFactory)
	return ctx
}
func unwrapXc(ctx context.Context) *factory.Factory {
	return ctx.Value(ContextXc).(*factory.Factory)
}
func wrapChain(ctx context.Context, chain *xc.ChainConfig) context.Context {
	ctx = context.WithValue(ctx, ContextChain, chain)
	return ctx
}
func unwrapChain(ctx context.Context) *xc.ChainConfig {
	return ctx.Value(ContextChain).(*xc.ChainConfig)
}

type RpcArgs struct {
	// Config         *tconfig.Connector
	Rpc            string
	Chain          string
	VerbosityCount int
	NotMainnet     bool
	Provider       string
	ApiKey         string
	ConfigPath     string

	Overrides map[string]*ChainOverride
}

func AddRpcArgs(cmd *cobra.Command) {
	cmd.PersistentFlags().String("config", "", "Path to treasury.toml configuration file.")
	cmd.PersistentFlags().String("rpc", "", "RPC url to use. Optional.")
	cmd.PersistentFlags().String("chain", "", "Chain to use. Required.")
	cmd.PersistentFlags().String("api-key", "", "Api key to use for client (may set CORDIAL_API_KEY).")
	cmd.PersistentFlags().String("provider", "", "Provider to use for chain client.  Only valid for BTC chains.")
	cmd.PersistentFlags().CountP("verbose", "v", "Set verbosity.")
	cmd.PersistentFlags().Bool("not-mainnet", false, "Do not use mainnets, instead use a test or dev network.")
}

func RpcArgsFromCmd(cmd *cobra.Command) (*RpcArgs, error) {
	config, _ := cmd.Flags().GetString("config")

	chain, _ := cmd.Flags().GetString("chain")
	rpc, _ := cmd.Flags().GetString("rpc")
	if chain == "" {
		return nil, fmt.Errorf("--chain required")
	}
	count, _ := cmd.Flags().GetCount("verbose")
	notmainnet, _ := cmd.Flags().GetBool("not-mainnet")
	provider, _ := cmd.Flags().GetString("provider")
	apikey, _ := cmd.Flags().GetString("api-key")
	if apikey == "" {
		apikey = os.Getenv("CORDIAL_API_KEY")
		if apikey == "" {
			// alias
			apikey = os.Getenv("TREASURY_API_KEY")
		}
	}

	return &RpcArgs{
		Chain:          chain,
		Rpc:            rpc,
		VerbosityCount: count,
		NotMainnet:     notmainnet,
		Provider:       provider,
		ApiKey:         apikey,
		ConfigPath:     config,
		Overrides:      map[string]*ChainOverride{},
	}, nil
}

func CmdXc() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "xc",
		Short:        "Manually interact with blockchains",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			args, err := RpcArgsFromCmd(cmd)
			if err != nil {
				return err
			}
			if args.VerbosityCount == 0 {
				logrus.SetLevel(logrus.WarnLevel)
			}
			if args.VerbosityCount == 1 {
				logrus.SetLevel(logrus.InfoLevel)
			}
			if args.VerbosityCount == 2 {
				logrus.SetLevel(logrus.DebugLevel)
			}
			if args.VerbosityCount >= 3 {
				logrus.SetLevel(logrus.TraceLevel)
			}
			if args.ConfigPath != "" {
				// currently only way to set config file is via env
				_ = os.Setenv(constants.ConfigEnv, args.ConfigPath)
			}

			xcFactory := factory.NewDefaultFactory()
			if args.NotMainnet {
				xcFactory = factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
			}
			var nativeAsset xc.NativeAsset
			for _, chainOption := range xc.NativeAssetList {
				if strings.EqualFold(string(chainOption), args.Chain) {
					nativeAsset = chainOption
				}
			}
			if nativeAsset == "" {
				return fmt.Errorf("invalid chain: %s\noptions: %v", args.Chain, xc.NativeAssetList)
			}

			if args.Rpc != "" {
				if existing, ok := args.Overrides[strings.ToLower(args.Chain)]; ok {
					existing.Rpc = args.Rpc
				} else {
					args.Overrides[strings.ToLower(args.Chain)] = &ChainOverride{
						Rpc: args.Rpc,
					}
				}
			}
			OverwriteCrosschainSettings(args.Overrides, xcFactory)
			if nil == cmd.Context() {
				cmd.SetContext(context.Background())
			}
			chainConfig, err := xcFactory.GetAssetConfig("", nativeAsset)
			if err != nil {
				return err
			}
			chain := chainConfig.(*xc.ChainConfig)
			if args.NotMainnet {
				chain.GetAllClients()[0].Network = "!mainnet"
				// needed for bitcoin chains
				chain.Net = "testnet"
			}
			if args.Provider != "" {
				chain.Provider = args.Provider
			}
			if args.ApiKey != "" {
				chain.AuthSecret = args.ApiKey
			}
			ctx := wrapXc(cmd.Context(), xcFactory)
			ctx = wrapChain(ctx, chain)
			logrus.WithFields(logrus.Fields{
				"rpc":     chain.GetAllClients()[0].URL,
				"network": chain.GetAllClients()[0].Network,
				"chain":   chain.Chain,
			}).Info("chain")
			cmd.SetContext(ctx)
			return nil
		},
	}
	AddRpcArgs(cmd)

	cmd.AddCommand(CmdRpcBalance())
	cmd.AddCommand(CmdTxInput())
	cmd.AddCommand(CmdTxInfo())
	cmd.AddCommand(CmdTxTransfer())
	cmd.AddCommand(CmdAddress())
	cmd.AddCommand(CmdChains())

	return cmd
}

func assetConfig(chain *xc.ChainConfig, contractMaybe string, decimals int32) xc.ITask {
	if contractMaybe != "" {
		token := xc.TokenAssetConfig{
			Contract:    contractMaybe,
			Chain:       chain.Chain,
			ChainConfig: chain,
			Decimals:    decimals,
		}
		return &token
	} else {
		return chain
	}
}

func main() {
	rootCmd := CmdXc()
	_ = rootCmd.Execute()
}
