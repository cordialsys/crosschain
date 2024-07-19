package setup

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
const ContextStakingArgs RpcContextKey = "staking"
const ContextChain RpcContextKey = "chain"

func WrapXc(ctx context.Context, xcFactory *factory.Factory) context.Context {
	ctx = context.WithValue(ctx, ContextXc, xcFactory)
	return ctx
}

func WrapStakingArgs(ctx context.Context, args *StakingArgs) context.Context {
	ctx = context.WithValue(ctx, ContextStakingArgs, args)
	return ctx
}
func WrapChain(ctx context.Context, chain *xc.ChainConfig) context.Context {
	ctx = context.WithValue(ctx, ContextChain, chain)
	return ctx
}
func UnwrapXc(ctx context.Context) *factory.Factory {
	return ctx.Value(ContextXc).(*factory.Factory)
}

func UnwrapStakingArgs(ctx context.Context) *StakingArgs {
	return ctx.Value(ContextStakingArgs).(*StakingArgs)
}

func UnwrapChain(ctx context.Context) *xc.ChainConfig {
	return ctx.Value(ContextChain).(*xc.ChainConfig)
}

func ConfigureLogger(args *RpcArgs) {
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
}

func LoadFactory(rcpArgs *RpcArgs) (*factory.Factory, error) {
	if rcpArgs.ConfigPath != "" {
		// currently only way to set config file is via env
		_ = os.Setenv(constants.ConfigEnv, rcpArgs.ConfigPath)
	}
	xcFactory := factory.NewDefaultFactory()
	if rcpArgs.NotMainnet {
		xcFactory = factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	}

	if rcpArgs.Rpc != "" {
		if existing, ok := rcpArgs.Overrides[strings.ToLower(rcpArgs.Chain)]; ok {
			existing.Rpc = rcpArgs.Rpc
		} else {
			rcpArgs.Overrides[strings.ToLower(rcpArgs.Chain)] = &ChainOverride{
				Rpc: rcpArgs.Rpc,
			}
		}
	}
	OverwriteCrosschainSettings(rcpArgs.Overrides, xcFactory)
	return xcFactory, nil
}
func LoadChain(xcFactory *factory.Factory, chain string) (*xc.ChainConfig, error) {
	var nativeAsset xc.NativeAsset
	for _, chainOption := range xc.NativeAssetList {
		if strings.EqualFold(string(chainOption), chain) {
			nativeAsset = chainOption
		}
	}
	if nativeAsset == "" {
		return nil, fmt.Errorf("invalid chain: %s\noptions: %v", chain, xc.NativeAssetList)
	}

	chainConfig, err := xcFactory.GetAssetConfig("", nativeAsset)
	if err != nil {
		return nil, err
	}
	chainCfg := chainConfig.(*xc.ChainConfig)
	return chainCfg, nil
}
func OverrideChainSettings(chain *xc.ChainConfig, args *RpcArgs) {
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
}

func CreateContext(xcFactory *factory.Factory, chain *xc.ChainConfig) context.Context {
	ctx := context.Background()
	ctx = WrapXc(ctx, xcFactory)
	ctx = WrapChain(ctx, chain)
	return ctx
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
	cmd.PersistentFlags().String("api-key", "", "Api key to use for client (may set API_KEY).")
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
		if apikey == "" {
			// alias
			apikey = os.Getenv("API_KEY")
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

type StakingArgs struct {
	KilnApi   string
	ApiKey    string
	AccountId string
	Amount    xc.AmountHumanReadable
}

func AddStakingArgs(cmd *cobra.Command) {
	cmd.PersistentFlags().String("kiln-api", "", "Override base URL for Kiln API.")
	cmd.PersistentFlags().String("staking-api-key", "", "API key to use for staking provider (may set STAKING_API_KEY).")
	cmd.PersistentFlags().String("account", "", "Account ID to stake into, if applicable.")
	cmd.PersistentFlags().String("amount", "", "Decimal amount to stake or unstake.")
}

func StakingArgsFromCmd(cmd *cobra.Command) (*StakingArgs, error) {
	kiln, err := cmd.Flags().GetString("kiln-api")
	if err != nil {
		return nil, err
	}
	accountId, err := cmd.Flags().GetString("account")
	if err != nil {
		return nil, err
	}

	apiKey, err := cmd.Flags().GetString("staking-api-key")
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		apiKey = os.Getenv("STAKING_API_KEY")
	}
	amount, err := cmd.Flags().GetString("amount")
	if err != nil {
		return nil, err
	}

	dec, _ := xc.NewAmountHumanReadableFromStr("0")
	if amount != "" {
		dec, err = xc.NewAmountHumanReadableFromStr(amount)
		if err != nil {
			return nil, err
		}
	}

	return &StakingArgs{
		KilnApi:   kiln,
		ApiKey:    apiKey,
		AccountId: accountId,
		Amount:    dec,
	}, nil
}

func OverrideStakingArgs(staking *StakingArgs, xcFactory *factory.Factory) {
	if staking.KilnApi == "" {
		if xcFactory.GetNetworkSelector() == xc.NotMainnets {
			staking.KilnApi = "https://api.testnet.kiln.fi"
		} else {
			staking.KilnApi = "https://api.kiln.fi"
		}
	}
}
