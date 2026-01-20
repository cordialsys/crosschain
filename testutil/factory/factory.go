package testutil

import (
	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/factory"
	factoryconfig "github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/signer"
)

// TestFactory for unit tests
type TestFactory struct {
	DefaultFactory factory.FactoryContext

	NewClientFunc               func(asset *xc.ChainConfig) (xclient.Client, error)
	NewTxBuilderFunc            func(asset *xc.ChainBaseConfig) (builder.FullTransferBuilder, error)
	NewSignerFunc               func(asset *xc.ChainBaseConfig) (*signer.Signer, error)
	NewAddressBuilderFunc       func(asset *xc.ChainBaseConfig) (xc.AddressBuilder, error)
	GetAddressFromPublicKeyFunc func(asset *xc.ChainBaseConfig, publicKey []byte) (xc.Address, error)
}

var _ factory.FactoryContext = &TestFactory{}

// NewClient creates a new Client
func (f *TestFactory) NewClient(asset *xc.ChainConfig) (xclient.Client, error) {
	if f.NewClientFunc != nil {
		return f.NewClientFunc(asset)
	}
	return f.DefaultFactory.NewClient(asset)
}

// NewTxBuilder creates a new TxBuilder
func (f *TestFactory) NewTxBuilder(asset *xc.ChainBaseConfig) (builder.FullTransferBuilder, error) {
	if f.NewTxBuilderFunc != nil {
		return f.NewTxBuilderFunc(asset)
	}
	return f.DefaultFactory.NewTxBuilder(asset)
}

// NewSigner creates a new Signer
func (f *TestFactory) NewSigner(asset *xc.ChainBaseConfig, secret string, options ...xcaddress.AddressOption) (*signer.Signer, error) {
	if f.NewSignerFunc != nil {
		return f.NewSignerFunc(asset)
	}
	return f.DefaultFactory.NewSigner(asset, secret, options...)
}

// NewAddressBuilder creates a new AddressBuilder
func (f *TestFactory) NewAddressBuilder(asset *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	if f.NewAddressBuilderFunc != nil {
		return f.NewAddressBuilderFunc(asset)
	}
	return f.DefaultFactory.NewAddressBuilder(asset)

}

// MarshalTxInput marshalls a TxInput struct
func (f *TestFactory) MarshalTxInput(input xc.TxInput) ([]byte, error) {
	return f.DefaultFactory.MarshalTxInput(input)
}

// UnmarshalTxInput unmarshalls data into a TxInput struct
func (f *TestFactory) UnmarshalTxInput(data []byte) (xc.TxInput, error) {
	return f.DefaultFactory.UnmarshalTxInput(data)
}

// GetAddressFromPublicKey returns an Address given a public key
func (f *TestFactory) GetAddressFromPublicKey(asset *xc.ChainBaseConfig, publicKey []byte, options ...xcaddress.AddressOption) (xc.Address, error) {
	if f.GetAddressFromPublicKeyFunc != nil {
		return f.GetAddressFromPublicKeyFunc(asset, publicKey)
	}
	return f.DefaultFactory.GetAddressFromPublicKey(asset, publicKey, options...)
}

// ConvertAmountToHuman converts an AmountBlockchain into AmountHumanReadable, dividing by the appropriate number of decimals
func (f *TestFactory) ConvertAmountToHuman(asset *xc.ChainConfig, blockchainAmount xc.AmountBlockchain) (xc.AmountHumanReadable, error) {
	return f.DefaultFactory.ConvertAmountToHuman(asset, blockchainAmount)
}

// ConvertAmountToBlockchain converts an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *TestFactory) ConvertAmountToBlockchain(asset *xc.ChainConfig, humanAmount xc.AmountHumanReadable) (xc.AmountBlockchain, error) {
	return f.DefaultFactory.ConvertAmountToBlockchain(asset, humanAmount)
}

// ConvertAmountStrToBlockchain converts a string representing an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *TestFactory) ConvertAmountStrToBlockchain(asset *xc.ChainConfig, humanAmountStr string) (xc.AmountBlockchain, error) {
	return f.DefaultFactory.ConvertAmountStrToBlockchain(asset, humanAmountStr)
}

// GetAssetConfig returns an AssetConfig by asset and native asset (chain)
func (f *TestFactory) GetChain(nativeAsset xc.NativeAsset) (*xc.ChainConfig, bool) {
	return f.DefaultFactory.GetChain(nativeAsset)
}

// Config returns the Config
func (f *TestFactory) GetConfig() factoryconfig.Config {
	return f.DefaultFactory.GetConfig()
}

// MustAddress coverts a string to Address, panic if error
func (f *TestFactory) MustAddress(asset *xc.ChainConfig, addressStr string) xc.Address {
	return f.DefaultFactory.MustAddress(asset, addressStr)
}

// MustAmountBlockchain coverts a string into AmountBlockchain, panic if error
func (f *TestFactory) MustAmountBlockchain(asset *xc.ChainConfig, humanAmountStr string) xc.AmountBlockchain {
	return f.DefaultFactory.MustAmountBlockchain(asset, humanAmountStr)

}

func (f *TestFactory) GetAllChains() []*xc.ChainConfig {
	return f.DefaultFactory.GetAllChains()
}

func (f *TestFactory) GetNetworkSelector() xc.NetworkSelector {
	return f.DefaultFactory.GetNetworkSelector()
}
func (f *TestFactory) NewStakingClient(stakingCfg *services.ServicesConfig, cfg *xc.ChainConfig, provider xc.StakingProvider) (xclient.StakingClient, error) {
	return f.DefaultFactory.NewStakingClient(stakingCfg, cfg, provider)
}

// NewDefaultFactory creates a new Factory
func NewDefaultFactory() TestFactory {
	f := factory.NewDefaultFactory()
	return TestFactory{
		DefaultFactory: f,
	}
}

func NewFactory(options factory.FactoryOptions) TestFactory {
	f := factory.NewFactory(&options)
	return TestFactory{
		DefaultFactory: f,
	}
}

// NewDefaultFactoryWithConfig creates a new Factory given a config map
func NewDefaultFactoryWithConfig(cfg *factoryconfig.Config) TestFactory {
	f := factory.NewDefaultFactoryWithConfig(cfg, nil)
	return TestFactory{
		DefaultFactory: f,
	}
}
