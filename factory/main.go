package factory

import (
	"fmt"

	. "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/address"
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
	NewClient(asset ITask) (xclient.Client, error)
	NewTxBuilder(asset ITask) (builder.FullTransferBuilder, error)
	NewSigner(asset ITask, secret string, interactive bool) (*signer.Signer, error)
	NewAddressBuilder(asset ITask) (AddressBuilder, error)

	MarshalTxInput(input TxInput) ([]byte, error)
	UnmarshalTxInput(data []byte) (TxInput, error)

	GetAddressFromPublicKey(asset ITask, publicKey []byte) (Address, error)

	MustAmountBlockchain(asset ITask, humanAmountStr string) AmountBlockchain
	MustAddress(asset ITask, addressStr string) Address

	ConvertAmountToHuman(asset ITask, blockchainAmount AmountBlockchain) (AmountHumanReadable, error)
	ConvertAmountToBlockchain(asset ITask, humanAmount AmountHumanReadable) (AmountBlockchain, error)
	ConvertAmountStrToBlockchain(asset ITask, humanAmountStr string) (AmountBlockchain, error)

	EnrichAssetConfig(partialCfg *TokenAssetConfig, nativeAsset NativeAsset) (*TokenAssetConfig, error)

	GetChain(nativeAsset NativeAsset) (*ChainConfig, bool)
	GetConfig() config.Config

	GetAllChains() []*ChainConfig

	GetNetworkSelector() NetworkSelector
	NewStakingClient(stakingCfg *services.ServicesConfig, cfg ITask, provider StakingProvider) (xclient.StakingClient, error)
}

// Factory is the main Factory implementation, holding the config
type Factory struct {
	AllChains   []*ChainConfig
	NoXcClients bool
	Config      *config.Config
}

var _ FactoryContext = &Factory{}

func (f *Factory) GetAllChains() []*ChainConfig {
	chains := make([]*ChainConfig, len(f.AllChains))
	copy(chains, f.AllChains)
	return chains
}

func (f *Factory) GetChain(nativeAsset NativeAsset) (*ChainConfig, bool) {
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

func (f *Factory) EnrichAssetConfig(partialCfg *TokenAssetConfig, nativeAsset NativeAsset) (*TokenAssetConfig, error) {
	chainCfg, found := f.GetChain(nativeAsset)
	if !found {
		return partialCfg, fmt.Errorf("unsupported chain: %s", nativeAsset)
	}
	// make copy so edits do not persist to local store
	partialCfg.ChainConfig = chainCfg
	partialCfg.Chain = nativeAsset

	return partialCfg, nil
}

// NewClient creates a new Client
func (f *Factory) NewClient(cfg ITask) (xclient.Client, error) {
	chainConfig := cfg.GetChain()
	if f.NoXcClients {
		if chainConfig.URL == "" {
			return nil, fmt.Errorf("no .URL set for %s chain, cannot construct client", chainConfig.Chain)
		}
		if chainConfig.Driver == DriverCrosschain {
			return nil, fmt.Errorf("cannot construct client for %s when no-xc-clients is set, and chain driver is %s", chainConfig.Chain, chainConfig.Driver)
		}
		return drivers.NewClient(cfg, chainConfig.Driver)
	}

	url, driver := chainConfig.ClientURL()
	switch driver {
	case DriverCrosschain:
		return remoteclient.NewClient(cfg, url, chainConfig.Auth2, chainConfig.CrosschainClient.Network)
	default:
		return drivers.NewClient(cfg, chainConfig.Driver)
	}
}

func (f *Factory) NewStakingClient(stakingCfg *services.ServicesConfig, cfg ITask, provider StakingProvider) (xclient.StakingClient, error) {
	chainConfig := cfg.GetChain()
	if !f.NoXcClients {
		url, driver := chainConfig.ClientURL()
		network := chainConfig.CrosschainClient.Network
		switch Driver(driver) {
		case DriverCrosschain:
			return remoteclient.NewStakingClient(cfg, url, chainConfig.Auth2, stakingCfg.GetApiSecret(provider), provider, network)
		}
	}
	return drivers.NewStakingClient(stakingCfg, cfg, provider)
}

// NewTxBuilder creates a new TxBuilder
func (f *Factory) NewTxBuilder(cfg ITask) (builder.FullTransferBuilder, error) {
	return drivers.NewTxBuilder(cfg)
}

func (f *Factory) NewStakingTxBuilder(cfg ITask) (builder.Staking, error) {
	txBuilder, err := f.NewTxBuilder(cfg)
	if err != nil {
		return nil, err
	}
	stakingBuilder, ok := txBuilder.(builder.Staking)
	if !ok {
		return nil, fmt.Errorf("currently staking transactions for %s is not supported", cfg.GetChain().Driver)
	}
	return stakingBuilder, nil
}

// NewSigner creates a new Signer
func (f *Factory) NewSigner(cfg ITask, secret string, interactive bool) (*signer.Signer, error) {
	return drivers.NewSigner(cfg, secret, interactive, address.OptionAlgorithm(f.Config.SignatureAlgorithm))
}

// NewAddressBuilder creates a new AddressBuilder
func (f *Factory) NewAddressBuilder(cfg ITask) (AddressBuilder, error) {
	return drivers.NewAddressBuilder(cfg, address.OptionAlgorithm(f.Config.SignatureAlgorithm))
}

// MarshalTxInput marshalls a TxInput struct
func (f *Factory) MarshalTxInput(input TxInput) ([]byte, error) {
	return drivers.MarshalTxInput(input)
}

// UnmarshalTxInput unmarshalls data into a TxInput struct
func (f *Factory) UnmarshalTxInput(data []byte) (TxInput, error) {
	return drivers.UnmarshalTxInput(data)
}

// GetAddressFromPublicKey returns an Address given a public key
func (f *Factory) GetAddressFromPublicKey(cfg ITask, publicKey []byte) (Address, error) {
	return getAddressFromPublicKey(
		cfg,
		publicKey,
		address.OptionAlgorithm(f.Config.SignatureAlgorithm),
	)
}

// ConvertAmountToHuman converts an AmountBlockchain into AmountHumanReadable, dividing by the appropriate number of decimals
func (f *Factory) ConvertAmountToHuman(cfg ITask, blockchainAmount AmountBlockchain) (AmountHumanReadable, error) {
	dec := cfg.GetDecimals()
	amount := blockchainAmount.ToHuman(dec)
	return amount, nil
}

// ConvertAmountToBlockchain converts an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *Factory) ConvertAmountToBlockchain(cfg ITask, humanAmount AmountHumanReadable) (AmountBlockchain, error) {
	dec := cfg.GetDecimals()
	amount := humanAmount.ToBlockchain(dec)
	return amount, nil
}

// ConvertAmountStrToBlockchain converts a string representing an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *Factory) ConvertAmountStrToBlockchain(cfg ITask, humanAmountStr string) (AmountBlockchain, error) {
	human, err := NewAmountHumanReadableFromStr(humanAmountStr)
	if err != nil {
		return AmountBlockchain{}, err
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
func (f *Factory) MustAddress(cfg ITask, addressStr string) Address {
	return Address(addressStr)
}

// MustAmountBlockchain coverts a string into AmountBlockchain, panic if error
func (f *Factory) MustAmountBlockchain(cfg ITask, humanAmountStr string) AmountBlockchain {
	res, err := f.ConvertAmountStrToBlockchain(cfg, humanAmountStr)
	if err != nil {
		panic(err)
	}
	return res
}

func (f *Factory) GetNetworkSelector() NetworkSelector {
	if f.Config.Network == config.Mainnet {
		return Mainnets
	}
	return NotMainnets
}

func getAddressFromPublicKey(cfg ITask, publicKey []byte, options ...address.AddressOption) (Address, error) {
	builder, err := drivers.NewAddressBuilder(cfg, options...)
	if err != nil {
		return "", err
	}
	return builder.GetAddressFromPublicKey(publicKey)
}

func CheckError(driver Driver, err error) errors.Status {
	if err, ok := err.(*errors.Error); ok {
		return err.Status
	}
	return drivers.CheckError(driver, err)
}
