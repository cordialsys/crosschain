package factory

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	remoteclient "github.com/cordialsys/crosschain/chain/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
)

// FactoryContext is the main Factory interface
type FactoryContext interface {
	NewClient(asset xc.ITask) (xclient.Client, error)
	NewTxBuilder(asset *xc.ChainBaseConfig) (builder.FullTransferBuilder, error)
	NewSigner(asset *xc.ChainBaseConfig, secret string, options ...xcaddress.AddressOption) (*signer.Signer, error)
	NewAddressBuilder(asset *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error)

	MarshalTxInput(input xc.TxInput) ([]byte, error)
	UnmarshalTxInput(data []byte) (xc.TxInput, error)

	GetAddressFromPublicKey(asset *xc.ChainBaseConfig, publicKey []byte, options ...xcaddress.AddressOption) (xc.Address, error)

	MustAmountBlockchain(asset xc.ITask, humanAmountStr string) xc.AmountBlockchain
	MustAddress(asset xc.ITask, addressStr string) xc.Address

	ConvertAmountToHuman(asset xc.ITask, blockchainAmount xc.AmountBlockchain) (xc.AmountHumanReadable, error)
	ConvertAmountToBlockchain(asset xc.ITask, humanAmount xc.AmountHumanReadable) (xc.AmountBlockchain, error)
	ConvertAmountStrToBlockchain(asset xc.ITask, humanAmountStr string) (xc.AmountBlockchain, error)

	GetChain(nativeAsset xc.NativeAsset) (*xc.ChainConfig, bool)
	GetConfig() config.Config

	GetAllChains() []*xc.ChainConfig

	GetNetworkSelector() xc.NetworkSelector
	NewStakingClient(stakingCfg *services.ServicesConfig, cfg xc.ITask, provider xc.StakingProvider) (xclient.StakingClient, error)
}

// Factory is the main Factory implementation, holding the config
type Factory struct {
	AllChains   []*xc.ChainConfig
	NoXcClients bool
	Config      *config.Config
}

var _ FactoryContext = &Factory{}

func (f *Factory) GetAllChains() []*xc.ChainConfig {
	chains := make([]*xc.ChainConfig, len(f.AllChains))
	copy(chains, f.AllChains)
	return chains
}

func (f *Factory) GetChain(nativeAsset xc.NativeAsset) (*xc.ChainConfig, bool) {
	for _, chain := range f.AllChains {
		if chain.Chain == nativeAsset {
			return chain, true
		}
	}
	return nil, false
}

func (f *Factory) GetConfig() config.Config {
	return *f.Config
}

// NewClient creates a new Client
func (f *Factory) NewClient(cfg xc.ITask) (xclient.Client, error) {
	chainConfig := cfg.GetChain()
	if f.NoXcClients {
		if chainConfig.URL == "" {
			return nil, fmt.Errorf("no .URL set for %s chain, cannot construct client", chainConfig.Chain)
		}
		if chainConfig.Driver == xc.DriverCrosschain {
			return nil, fmt.Errorf("cannot construct client for %s when no-xc-clients is set, and chain driver is %s", chainConfig.Chain, chainConfig.Driver)
		}
		return drivers.NewClient(cfg, chainConfig.Driver)
	}

	url, driver := chainConfig.ClientURL()
	switch driver {
	case xc.DriverCrosschain:
		return remoteclient.NewClient(cfg, url, chainConfig.Auth2, chainConfig.CrosschainClient.Network, f.Config.HttpTimeout)
	default:
		return drivers.NewClient(cfg, chainConfig.Driver)
	}
}

func (f *Factory) NewStakingClient(stakingCfg *services.ServicesConfig, cfg xc.ITask, provider xc.StakingProvider) (xclient.StakingClient, error) {
	chainConfig := cfg.GetChain()
	if !f.NoXcClients {
		url, driver := chainConfig.ClientURL()
		network := chainConfig.CrosschainClient.Network
		switch xc.Driver(driver) {
		case xc.DriverCrosschain:
			return remoteclient.NewStakingClient(cfg, url, chainConfig.Auth2, stakingCfg.GetApiSecret(provider), provider, network, f.Config.HttpTimeout)
		}
	}
	return drivers.NewStakingClient(stakingCfg, cfg, provider)
}

// NewTxBuilder creates a new TxBuilder
func (f *Factory) NewTxBuilder(cfg *xc.ChainBaseConfig) (builder.FullTransferBuilder, error) {
	return drivers.NewTxBuilder(cfg)
}

func (f *Factory) NewStakingTxBuilder(cfg *xc.ChainBaseConfig) (builder.Staking, error) {
	txBuilder, err := f.NewTxBuilder(cfg)
	if err != nil {
		return nil, err
	}
	stakingBuilder, ok := txBuilder.(builder.Staking)
	if !ok {
		return nil, fmt.Errorf("currently staking transactions for %s is not supported", cfg.Driver)
	}
	return stakingBuilder, nil
}

// NewSigner creates a new Signer
func (f *Factory) NewSigner(cfg *xc.ChainBaseConfig, secret string, options ...xcaddress.AddressOption) (*signer.Signer, error) {
	return drivers.NewSigner(cfg, secret, options...)
}

// NewAddressBuilder creates a new AddressBuilder
func (f *Factory) NewAddressBuilder(cfg *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	return drivers.NewAddressBuilder(cfg, options...)
}

// MarshalTxInput marshalls a TxInput struct
func (f *Factory) MarshalTxInput(input xc.TxInput) ([]byte, error) {
	return drivers.MarshalTxInput(input)
}

// UnmarshalTxInput unmarshalls data into a TxInput struct
func (f *Factory) UnmarshalTxInput(data []byte) (xc.TxInput, error) {
	return drivers.UnmarshalTxInput(data)
}

// Simulate real TxInput life time, by roundtriping it throught factory Marshal/Unmarshal methods
func (f *Factory) TxInputRoundtrip(input xc.TxInput) (xc.TxInput, error) {
	inputBytes, err := f.MarshalTxInput(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx input: %w", err)
	}

	return f.UnmarshalTxInput(inputBytes)
}

// GetAddressFromPublicKey returns an Address given a public key
func (f *Factory) GetAddressFromPublicKey(cfg *xc.ChainBaseConfig, publicKey []byte, options ...xcaddress.AddressOption) (xc.Address, error) {
	return getAddressFromPublicKey(
		cfg,
		publicKey,
		options...,
	)
}

// ConvertAmountToHuman converts an AmountBlockchain into AmountHumanReadable, dividing by the appropriate number of decimals
func (f *Factory) ConvertAmountToHuman(cfg xc.ITask, blockchainAmount xc.AmountBlockchain) (xc.AmountHumanReadable, error) {
	dec := cfg.GetDecimals()
	amount := blockchainAmount.ToHuman(dec)
	return amount, nil
}

// ConvertAmountToBlockchain converts an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *Factory) ConvertAmountToBlockchain(cfg xc.ITask, humanAmount xc.AmountHumanReadable) (xc.AmountBlockchain, error) {
	dec := cfg.GetDecimals()
	amount := humanAmount.ToBlockchain(dec)
	return amount, nil
}

// ConvertAmountStrToBlockchain converts a string representing an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *Factory) ConvertAmountStrToBlockchain(cfg xc.ITask, humanAmountStr string) (xc.AmountBlockchain, error) {
	human, err := xc.NewAmountHumanReadableFromStr(humanAmountStr)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	return f.ConvertAmountToBlockchain(cfg, human)
}

// func (f *Factory) cfgFromAssetByContract(contract string, nativeAsset NativeAsset) (ITask, error) {
// 	var res ITask
// 	contract = normalize.NormalizeAddressString(contract, nativeAsset)
// 	f.AllAssets.Range(func(key, value interface{}) bool {
// 		cfg := value.(ITask)
// 		chain := cfg.GetChain().Chain
// 		cfgContract := ""
// 		switch asset := cfg.(type) {
// 		case *TokenAssetConfig:
// 			cfgContract = normalize.NormalizeAddressString(asset.Contract, nativeAsset)
// 		case *ChainConfig:
// 		}
// 		if chain == nativeAsset && cfgContract == contract {
// 			res = value.(ITask)
// 			return false
// 		}
// 		return true
// 	})
// 	if res != nil {
// 		return f.cfgFromAsset(res.ID())
// 	} else {
// 		if f.callbackGetAssetConfigByContract != nil {
// 			return f.callbackGetAssetConfigByContract(contract, nativeAsset)
// 		}
// 	}
// 	return &TokenAssetConfig{}, fmt.Errorf("unknown contract: '%s'", contract)
// }

// // GetAssetConfigByContract returns an AssetConfig by contract and native asset (chain)
// func (f *Factory) GetAssetConfigByContract(contract string, nativeAsset NativeAsset) (ITask, error) {
// 	return f.cfgFromAssetByContract(contract, nativeAsset)
// }

// MustAddress coverts a string to Address, panic if error
func (f *Factory) MustAddress(cfg xc.ITask, addressStr string) xc.Address {
	return xc.Address(addressStr)
}

// MustAmountBlockchain coverts a string into AmountBlockchain, panic if error
func (f *Factory) MustAmountBlockchain(cfg xc.ITask, humanAmountStr string) xc.AmountBlockchain {
	res, err := f.ConvertAmountStrToBlockchain(cfg, humanAmountStr)
	if err != nil {
		panic(err)
	}
	return res
}

func (f *Factory) GetNetworkSelector() xc.NetworkSelector {
	if f.Config.Network == config.Mainnet {
		return xc.Mainnets
	}
	return xc.NotMainnets
}

func getAddressFromPublicKey(cfg *xc.ChainBaseConfig, publicKey []byte, options ...xcaddress.AddressOption) (xc.Address, error) {
	builder, err := drivers.NewAddressBuilder(cfg, options...)
	if err != nil {
		return "", err
	}
	return builder.GetAddressFromPublicKey(publicKey)
}

func CheckError(driver xc.Driver, err error) errors.Status {
	if err, ok := err.(*errors.Error); ok {
		return err.Status
	}
	return drivers.CheckError(driver, err)
}
