package testutil

import (
	xc "github.com/cordialsys/crosschain"
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

	NewClientFunc               func(asset xc.ITask) (xclient.Client, error)
	NewTxBuilderFunc            func(asset xc.ITask) (builder.FullTransferBuilder, error)
	NewSignerFunc               func(asset xc.ITask) (*signer.Signer, error)
	NewAddressBuilderFunc       func(asset xc.ITask) (xc.AddressBuilder, error)
	GetAddressFromPublicKeyFunc func(asset xc.ITask, publicKey []byte) (xc.Address, error)
}

var _ factory.FactoryContext = &TestFactory{}

// NewClient creates a new Client
func (f *TestFactory) NewClient(asset xc.ITask) (xclient.Client, error) {
	if f.NewClientFunc != nil {
		return f.NewClientFunc(asset)
	}
	return f.DefaultFactory.NewClient(asset)
}

// NewTxBuilder creates a new TxBuilder
func (f *TestFactory) NewTxBuilder(asset xc.ITask) (builder.FullTransferBuilder, error) {
	if f.NewTxBuilderFunc != nil {
		return f.NewTxBuilderFunc(asset)
	}
	return f.DefaultFactory.NewTxBuilder(asset)
}

// NewSigner creates a new Signer
func (f *TestFactory) NewSigner(asset xc.ITask, secret string) (*signer.Signer, error) {
	if f.NewSignerFunc != nil {
		return f.NewSignerFunc(asset)
	}
	return f.DefaultFactory.NewSigner(asset.GetChain(), secret)
}

// NewAddressBuilder creates a new AddressBuilder
func (f *TestFactory) NewAddressBuilder(asset xc.ITask) (xc.AddressBuilder, error) {
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
func (f *TestFactory) GetAddressFromPublicKey(asset xc.ITask, publicKey []byte) (xc.Address, error) {
	if f.GetAddressFromPublicKeyFunc != nil {
		return f.GetAddressFromPublicKeyFunc(asset, publicKey)
	}
	return f.DefaultFactory.GetAddressFromPublicKey(asset, publicKey)
}

// GetAllPossibleAddressesFromPublicKey returns all PossibleAddress(es) given a public key
func (f *TestFactory) GetAllPossibleAddressesFromPublicKey(asset xc.ITask, publicKey []byte) ([]xc.PossibleAddress, error) {
	if f.GetAddressFromPublicKeyFunc != nil {
		return f.GetAllPossibleAddressesFromPublicKey(asset, publicKey)
	}
	return f.DefaultFactory.GetAllPossibleAddressesFromPublicKey(asset, publicKey)
}

// ConvertAmountToHuman converts an AmountBlockchain into AmountHumanReadable, dividing by the appropriate number of decimals
func (f *TestFactory) ConvertAmountToHuman(asset xc.ITask, blockchainAmount xc.AmountBlockchain) (xc.AmountHumanReadable, error) {
	return f.DefaultFactory.ConvertAmountToHuman(asset, blockchainAmount)
}

// ConvertAmountToBlockchain converts an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *TestFactory) ConvertAmountToBlockchain(asset xc.ITask, humanAmount xc.AmountHumanReadable) (xc.AmountBlockchain, error) {
	return f.DefaultFactory.ConvertAmountToBlockchain(asset, humanAmount)
}

// ConvertAmountStrToBlockchain converts a string representing an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *TestFactory) ConvertAmountStrToBlockchain(asset xc.ITask, humanAmountStr string) (xc.AmountBlockchain, error) {
	return f.DefaultFactory.ConvertAmountStrToBlockchain(asset, humanAmountStr)
}

// GetAssetConfig returns an AssetConfig by asset and native asset (chain)
func (f *TestFactory) GetAssetConfig(asset string, nativeAsset xc.NativeAsset) (xc.ITask, error) {
	return f.DefaultFactory.GetAssetConfig(asset, nativeAsset)
}

// GetTaskConfig returns an AssetConfig by task name and assetID
func (f *TestFactory) GetTaskConfig(taskName string, assetID xc.AssetID) (xc.ITask, error) {
	return f.DefaultFactory.GetTaskConfig(taskName, assetID)
}

func (f *TestFactory) GetTaskConfigByNameSrcDstAssetIDs(taskName string, srcAssetID xc.AssetID, dstAssetID xc.AssetID) (xc.ITask, error) {
	return f.DefaultFactory.GetTaskConfigByNameSrcDstAssetIDs(taskName, srcAssetID, dstAssetID)
}

// GetTaskConfigBySrcDstAssets returns an AssetConfig by source and destination assets
func (f *TestFactory) GetTaskConfigBySrcDstAssets(srcAsset xc.ITask, dstAsset xc.ITask) ([]xc.ITask, error) {
	return f.DefaultFactory.GetTaskConfigBySrcDstAssets(srcAsset, srcAsset)
}

// GetMultiAssetConfig returns an AssetConfig by source and destination assetIDs
func (f *TestFactory) GetMultiAssetConfig(srcAssetID xc.AssetID, dstAssetID xc.AssetID) ([]xc.ITask, error) {
	return f.DefaultFactory.GetMultiAssetConfig(srcAssetID, dstAssetID)
}

// GetAssetConfigByContract returns an AssetConfig by contract and native asset (chain)
func (f *TestFactory) GetAssetConfigByContract(contract string, nativeAsset xc.NativeAsset) (xc.ITask, error) {
	return f.DefaultFactory.GetAssetConfigByContract(contract, nativeAsset)
}

// EnrichAssetConfig augments a partial AssetConfig, for example if some info is stored in a db and other in a config file
func (f *TestFactory) EnrichAssetConfig(partialCfg *xc.TokenAssetConfig) (*xc.TokenAssetConfig, error) {
	return f.DefaultFactory.EnrichAssetConfig(partialCfg)
}

// EnrichDestinations augments a TxInfo by resolving assets and amounts in TxInfo.Destinations
func (f *TestFactory) EnrichDestinations(activity xc.ITask, txInfo xc.LegacyTxInfo) (xc.LegacyTxInfo, error) {
	return f.DefaultFactory.EnrichDestinations(activity, txInfo)
}

func (f *TestFactory) RegisterGetAssetConfigCallback(callback func(assetID xc.AssetID) (xc.ITask, error)) {
	f.DefaultFactory.RegisterGetAssetConfigCallback(callback)
}

func (f *TestFactory) UnregisterGetAssetConfigCallback() {
	f.DefaultFactory.UnregisterGetAssetConfigCallback()
}

func (f *TestFactory) RegisterGetAssetConfigByContractCallback(callback func(contract string, nativeAsset xc.NativeAsset) (xc.ITask, error)) {
	f.DefaultFactory.RegisterGetAssetConfigByContractCallback(callback)
}

func (f *TestFactory) UnregisterGetAssetConfigByContractCallback() {
	f.DefaultFactory.UnregisterGetAssetConfigByContractCallback()
}

// PutAssetConfig adds an AssetConfig to the current Config cache
func (f *TestFactory) PutAssetConfig(config xc.ITask) (xc.ITask, error) {
	return f.DefaultFactory.PutAssetConfig(config)
}

// Config returns the Config
func (f *TestFactory) GetConfig() factoryconfig.Config {
	return f.DefaultFactory.GetConfig()
}

// MustAddress coverts a string to Address, panic if error
func (f *TestFactory) MustAddress(asset xc.ITask, addressStr string) xc.Address {
	return f.DefaultFactory.MustAddress(asset, addressStr)
}

// MustAmountBlockchain coverts a string into AmountBlockchain, panic if error
func (f *TestFactory) MustAmountBlockchain(asset xc.ITask, humanAmountStr string) xc.AmountBlockchain {
	return f.DefaultFactory.MustAmountBlockchain(asset, humanAmountStr)

}

func (f *TestFactory) GetAllAssets() []xc.ITask {
	return f.DefaultFactory.GetAllAssets()
}

func (f *TestFactory) GetAllTasks() []*xc.TaskConfig {
	return f.DefaultFactory.GetAllTasks()
}

func (f *TestFactory) GetNetworkSelector() xc.NetworkSelector {
	return f.DefaultFactory.GetNetworkSelector()
}
func (f *TestFactory) NewStakingClient(stakingCfg *services.ServicesConfig, cfg xc.ITask, provider xc.StakingProvider) (xclient.StakingClient, error) {
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
