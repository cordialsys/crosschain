package setup

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cometbft/cometbft/types/time"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type RpcContextKey string

const ContextXc RpcContextKey = "xc"
const ContextStakingArgs RpcContextKey = "staking-args"
const ContextStakingConfig RpcContextKey = "staking-config"
const ContextChain RpcContextKey = "chain"

func WrapXc(ctx context.Context, xcFactory *factory.Factory) context.Context {
	ctx = context.WithValue(ctx, ContextXc, xcFactory)
	return ctx
}

func WrapStakingArgs(ctx context.Context, args *StakingArgs) context.Context {
	ctx = context.WithValue(ctx, ContextStakingArgs, args)
	return ctx
}
func WrapStakingConfig(ctx context.Context, args *services.ServicesConfig) context.Context {
	ctx = context.WithValue(ctx, ContextStakingConfig, args)
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
func UnwrapStakingConfig(ctx context.Context) *services.ServicesConfig {
	return ctx.Value(ContextStakingConfig).(*services.ServicesConfig)
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
	// if rcpArgs.ConfigPath != "" {
	// 	// currently only way to set config file is via env
	// 	_ = os.Setenv(constants.ConfigEnv, rcpArgs.ConfigPath)
	// }
	xcFactory := factory.NewDefaultFactory()
	if rcpArgs.NotMainnet {
		xcFactory = factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	}

	override, ok := rcpArgs.Overrides[strings.ToLower(rcpArgs.Chain)]
	if !ok {
		rcpArgs.Overrides[strings.ToLower(rcpArgs.Chain)] = &ChainOverride{}
		override = rcpArgs.Overrides[strings.ToLower(rcpArgs.Chain)]
	}

	if rcpArgs.Rpc != "" {
		override.Rpc = rcpArgs.Rpc
	}
	if rcpArgs.Network != "" {
		override.Network = rcpArgs.Network
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

	chainConfig, ok := xcFactory.GetChain(nativeAsset)
	if !ok {
		return nil, fmt.Errorf("chain not found: %v", nativeAsset)
	}
	return chainConfig, nil
}
func OverrideChainSettings(chain *xc.ChainConfig, args *RpcArgs) {
	if args.NotMainnet {
		chain.CrosschainClient.Network = "!mainnet"
		// needed for bitcoin chains
		chain.Network = "testnet"
	}
	if args.Provider != "" {
		chain.Provider = args.Provider
		chain.IndexerType = args.Provider
		chain.IndexerUrl = args.Rpc
	}
	if args.ApiKey != "" {
		if args.ApiKey == DefaultApiRef && chain.Auth2 != "" {
			// do not override existing config setting with default api-ref
		} else {
			chain.Auth2 = args.ApiKey
		}
	}
	if args.Network != "" {
		chain.Network = args.Network
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
	Algorithm      string
	VerbosityCount int
	NotMainnet     bool
	Provider       string
	Network        string
	ApiKey         config.Secret
	// ConfigPath     string
	UseLocalImplementation bool

	Overrides map[string]*ChainOverride
}

const DefaultApiRef = "env:API_KEY"
const DefaultTreasuryApiRef = "env:TREASURY_API_KEY"

func AddRpcArgs(cmd *cobra.Command) {
	// cmd.PersistentFlags().String("config", "", "Path to treasury.toml configuration file.")
	cmd.PersistentFlags().String("rpc", "", "RPC url to use. Optional.")
	cmd.PersistentFlags().String("chain", os.Getenv("XC_CHAIN"), "Chain to use (may set XC_CHAIN env var). Required.")
	cmd.PersistentFlags().String("algorithm", "", "Override default signing algorithm. Optional, used only by bitcoin.")
	cmd.PersistentFlags().String("api-key", DefaultApiRef, fmt.Sprintf("Secret reference for API key to use for RPC client (may also set %s).", DefaultTreasuryApiRef))
	cmd.PersistentFlags().String("rpc-provider", "", "Provider to use for RPC client.  Only valid for bitcoin chains.")
	cmd.PersistentFlags().String("network", "", "Network to use.  Only used for bitcoin chains.")
	cmd.PersistentFlags().CountP("verbose", "v", "Set verbosity.")
	cmd.PersistentFlags().Bool("not-mainnet", false, "Do not use mainnets, instead use a test or dev network.")
	cmd.PersistentFlags().Bool("local", false, "Use local client implementation(s) instead of using remote connector.cordialapis.com.")
}

func RpcArgsFromCmd(cmd *cobra.Command) (*RpcArgs, error) {
	// config, _ := cmd.Flags().GetString("config")

	chain, _ := cmd.Flags().GetString("chain")
	algorithm, _ := cmd.Flags().GetString("algorithm")
	rpc, _ := cmd.Flags().GetString("rpc")
	if chain == "" {
		return nil, fmt.Errorf("--chain required")
	}
	count, _ := cmd.Flags().GetCount("verbose")
	local, err := cmd.Flags().GetBool("local")
	if err != nil {
		return nil, err
	}
	notmainnet, _ := cmd.Flags().GetBool("not-mainnet")
	rpcProvider, _ := cmd.Flags().GetString("rpc-provider")
	apikey, _ := cmd.Flags().GetString("api-key")
	if apikey != "" && len(strings.Split(apikey, ":")) == 0 {
		return nil, fmt.Errorf("api-key must not be passed directly on command, instead you should use a reference (default is %s)", DefaultApiRef)
	}
	if apikey == DefaultApiRef {
		if val, _ := config.Secret(apikey).Load(); val == "" {
			if val, _ := config.Secret(DefaultTreasuryApiRef).Load(); val != "" {
				// use TREASURY_API_KEY if API_KEY is not set
				apikey = DefaultTreasuryApiRef
			}
		}
	}

	network, err := cmd.Flags().GetString("network")
	if err != nil {
		return nil, err
	}

	return &RpcArgs{
		Chain:                  chain,
		Algorithm:              algorithm,
		Rpc:                    rpc,
		VerbosityCount:         count,
		NotMainnet:             notmainnet,
		Provider:               rpcProvider,
		ApiKey:                 config.Secret(apikey),
		UseLocalImplementation: local,
		Network:                network,
		// ConfigPath:     config,
		Overrides: map[string]*ChainOverride{},
	}, nil
}

type StakingArgs struct {
	ConfigPath string
	AccountId  string
	Amount     xc.AmountHumanReadable
	Validator  string
	Provider   xc.StakingProvider
}

func (args *StakingArgs) ToBuilderOptions() []builder.BuilderOption {
	options := []builder.BuilderOption{}
	if args.Validator != "" {
		options = append(options, builder.OptionValidator(args.Validator))
	}
	if args.AccountId != "" {
		options = append(options, builder.OptionStakeAccount(args.AccountId))
	}
	options = append(options, builder.OptionTimestamp(time.Now().Unix()))
	return options
}

func (args *StakingArgs) BuilderOptionsWith(publicKey []byte) []builder.BuilderOption {
	options := args.ToBuilderOptions()
	options = append(options, builder.OptionPublicKey(publicKey))
	return options
}

func (args *StakingArgs) ToBalanceOptions() []client.StakedBalanceOption {
	options := []client.StakedBalanceOption{}
	if args.Validator != "" {
		options = append(options, client.StakeBalanceOptionValidator(args.Validator))
	}
	if args.AccountId != "" {
		options = append(options, client.StakeBalanceOptionAccount(args.AccountId))
	}
	return options
}

func AddStakingArgs(cmd *cobra.Command) {
	cmd.PersistentFlags().String("config", "", fmt.Sprintf("Staking client configuration to use (may set %s).", services.ConfigFileEnv))

	cmd.PersistentFlags().String("validator", "", "Validator address, if applicable.")
	cmd.PersistentFlags().String("account", "", "Account address or ID, if applicable.")
	cmd.PersistentFlags().String("amount", "", "Decimal amount to stake or unstake.")

	options := []string{}
	for _, v := range xc.SupportedStakingProviders {
		options = append(options, string(v))
	}
	cmd.PersistentFlags().String("provider", "native", fmt.Sprintf("Staking provider to use, if supported by the target chain (options: %v).", options))
}

func StakingArgsFromCmd(cmd *cobra.Command) (*StakingArgs, error) {

	accountId, err := cmd.Flags().GetString("account")
	if err != nil {
		return nil, err
	}
	validator, err := cmd.Flags().GetString("validator")
	if err != nil {
		return nil, err
	}

	providerInput, err := cmd.Flags().GetString("provider")
	if err != nil {
		return nil, err
	}
	var provider xc.StakingProvider
	for _, validProvider := range xc.SupportedStakingProviders {
		if strings.EqualFold(string(validProvider), providerInput) {
			provider = validProvider
			break
		}
	}
	if provider == "" {
		return nil, fmt.Errorf("invalid provider: %s", providerInput)
	}

	amount, err := cmd.Flags().GetString("amount")
	if err != nil {
		return nil, err
	}

	configPath, err := cmd.Flags().GetString("config")
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
		ConfigPath: configPath,
		AccountId:  accountId,
		Amount:     dec,
		Validator:  validator,
		Provider:   provider,
	}, nil
}
