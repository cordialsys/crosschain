package factory

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	. "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	remoteclient "github.com/cordialsys/crosschain/chain/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/cordialsys/crosschain/normalize"
)

// FactoryContext is the main Factory interface
type FactoryContext interface {
	NewClient(asset ITask) (xclient.Client, error)
	NewTxBuilder(asset ITask) (builder.FullTransferBuilder, error)
	NewSigner(asset ITask, secret string) (*signer.Signer, error)
	NewAddressBuilder(asset ITask) (AddressBuilder, error)

	MarshalTxInput(input TxInput) ([]byte, error)
	UnmarshalTxInput(data []byte) (TxInput, error)

	GetAddressFromPublicKey(asset ITask, publicKey []byte) (Address, error)
	GetAllPossibleAddressesFromPublicKey(asset ITask, publicKey []byte) ([]PossibleAddress, error)

	MustAmountBlockchain(asset ITask, humanAmountStr string) AmountBlockchain
	MustAddress(asset ITask, addressStr string) Address

	ConvertAmountToHuman(asset ITask, blockchainAmount AmountBlockchain) (AmountHumanReadable, error)
	ConvertAmountToBlockchain(asset ITask, humanAmount AmountHumanReadable) (AmountBlockchain, error)
	ConvertAmountStrToBlockchain(asset ITask, humanAmountStr string) (AmountBlockchain, error)

	EnrichAssetConfig(partialCfg *TokenAssetConfig) (*TokenAssetConfig, error)
	EnrichDestinations(asset ITask, txInfo LegacyTxInfo) (LegacyTxInfo, error)

	GetAssetConfig(asset string, nativeAsset NativeAsset) (ITask, error)
	GetAssetConfigByContract(contract string, nativeAsset NativeAsset) (ITask, error)
	PutAssetConfig(config ITask) (ITask, error)
	GetConfig() config.Config

	GetAllAssets() []ITask
	GetAllTasks() []*TaskConfig
	GetMultiAssetConfig(srcAssetID AssetID, dstAssetID AssetID) ([]ITask, error)
	GetTaskConfig(taskName string, assetID AssetID) (ITask, error)
	GetTaskConfigByNameSrcDstAssetIDs(taskName string, srcAssetID AssetID, dstAssetID AssetID) (ITask, error)
	GetTaskConfigBySrcDstAssets(srcAsset ITask, dstAsset ITask) ([]ITask, error)

	RegisterGetAssetConfigCallback(callback func(assetID AssetID) (ITask, error))
	UnregisterGetAssetConfigCallback()
	RegisterGetAssetConfigByContractCallback(callback func(contract string, nativeAsset NativeAsset) (ITask, error))
	UnregisterGetAssetConfigByContractCallback()

	GetNetworkSelector() NetworkSelector
	NewStakingClient(stakingCfg *services.ServicesConfig, cfg ITask, provider StakingProvider) (xclient.StakingClient, error)
}

// Factory is the main Factory implementation, holding the config
type Factory struct {
	AllAssets                        *sync.Map
	AllTasks                         []*TaskConfig
	AllPipelines                     []*PipelineConfig
	callbackGetAssetConfig           func(assetID AssetID) (ITask, error)
	callbackGetAssetConfigByContract func(contract string, nativeAsset NativeAsset) (ITask, error)
	NoXcClients                      bool
	Config                           *config.Config
}

var _ FactoryContext = &Factory{}

func (f *Factory) GetAllAssets() []ITask {
	tasks := []ITask{}
	f.AllAssets.Range(func(key, value any) bool {
		asset := value.(ITask)
		task, _ := f.cfgFromAsset(asset.ID())
		tasks = append(tasks, task)
		return true
	})
	// sort so it's deterministc
	sort.Slice(tasks, func(i, j int) bool {
		asset_i := tasks[i].GetChain()
		asset_j := tasks[j].GetChain()
		key1 := string(asset_i.ID()) + string(asset_i.Chain) + asset_i.ChainName
		key2 := string(asset_j.ID()) + string(asset_j.Chain) + asset_j.ChainName
		return key1 < key2
	})
	return tasks
}
func (f *Factory) GetAllTasks() []*TaskConfig {
	tasks := append([]*TaskConfig{}, f.AllTasks...)
	// sort so it's deterministc
	sort.Slice(tasks, func(i, j int) bool {
		key1 := tasks[i].Name
		key2 := tasks[j].Name
		return key1 < key2
	})
	return tasks
}

func (f *Factory) cfgFromAsset(assetID AssetID) (ITask, error) {
	cfgI, found := f.AllAssets.Load(assetID)
	if !found {
		if f.callbackGetAssetConfig != nil {
			return f.callbackGetAssetConfig(assetID)
		}
		return &ChainConfig{}, fmt.Errorf("could not lookup asset: '%s'", assetID)
	}
	if cfg, ok := cfgI.(*ChainConfig); ok {
		// native asset
		// cfg.Type = AssetTypeNative
		// cfg.Chain = cfg.Asset
		// cfg.NativeAsset = NativeAsset(cfg.Asset)
		return cfg, nil
	}
	if cfg, ok := cfgI.(*TokenAssetConfig); ok {
		// token
		cfg, _ = f.cfgEnrichToken(cfg)
		return cfg, nil
	}
	return &ChainConfig{}, fmt.Errorf("invalid asset: '%s'", assetID)
}

func (f *Factory) cfgFromAssetByContract(contract string, nativeAsset NativeAsset) (ITask, error) {
	var res ITask
	contract = normalize.NormalizeAddressString(contract, nativeAsset)
	f.AllAssets.Range(func(key, value interface{}) bool {
		cfg := value.(ITask)
		chain := cfg.GetChain().Chain
		cfgContract := ""
		switch asset := cfg.(type) {
		case *TokenAssetConfig:
			cfgContract = normalize.NormalizeAddressString(asset.Contract, nativeAsset)
		case *ChainConfig:
		}
		if chain == nativeAsset && cfgContract == contract {
			res = value.(ITask)
			return false
		}
		return true
	})
	if res != nil {
		return f.cfgFromAsset(res.ID())
	} else {
		if f.callbackGetAssetConfigByContract != nil {
			return f.callbackGetAssetConfigByContract(contract, nativeAsset)
		}
	}
	return &TokenAssetConfig{}, fmt.Errorf("unknown contract: '%s'", contract)
}

func (f *Factory) enrichTask(task *TaskConfig, srcAssetID AssetID, dstAssetID AssetID) (*TaskConfig, error) {
	dstAsset, err := f.cfgFromAsset(dstAssetID)
	if err != nil {
		return task, fmt.Errorf("unenriched task '%s' has invalid dst asset: '%s': %v", task.ID(), dstAssetID, err)
	}

	newTask := *task
	newTask.SrcAsset, _ = f.cfgFromAsset(srcAssetID)
	newTask.DstAsset = dstAsset
	return &newTask, nil
}

func (f *Factory) enrichTaskBySrcDstAssets(task *TaskConfig, srcAsset ITask, dstAsset ITask) (*TaskConfig, error) {
	task.SrcAsset = srcAsset
	task.DstAsset = dstAsset
	return task, nil
}

func (f *Factory) cfgFromTask(taskName string, assetIDIn AssetID) (ITask, error) {
	assetID := GetAssetIDFromAsset(string(assetIDIn), "")
	IsAllowedFunc := func(task *TaskConfig, assetID AssetID) (*TaskConfig, error) {
		allowed := false
		dstAssetID := AssetID("")
		for _, entry := range task.AllowList {
			if entry.Src == "*" {
				allowed = true
				dstAssetID = assetID
				break
			}
			if entry.Src == assetID {
				allowed = true
				dstAssetID = entry.Dst
				break
			}
		}
		if !allowed {
			return task, fmt.Errorf("task '%s' not allowed: '%s'", taskName, assetID)
		}
		return f.enrichTask(task, assetID, dstAssetID)
	}

	assetCfg, err := f.cfgFromAsset(assetID)
	if taskName == "" {
		return assetCfg, err
	}

	task, err := f.findTask(taskName)
	if err != nil {
		return &TaskConfig{}, fmt.Errorf("invalid task: '%s'", taskName)
	}

	res, err := IsAllowedFunc(task, assetID)
	return res, err
}

func (f *Factory) findTask(taskName string) (*TaskConfig, error) {
	// TODO switch to map
	for _, task := range f.AllTasks {
		if string(task.ID()) == taskName {
			return task, nil
		}
	}
	return &TaskConfig{}, fmt.Errorf("invalid task: '%s'", taskName)
}

func (f *Factory) getTaskConfigBySrcDstAssets(srcAsset ITask, dstAsset ITask) ([]ITask, error) {
	srcAssetID := srcAsset.ID()
	dstAssetID := dstAsset.ID()
	for _, task := range f.AllTasks {
		for _, entry := range task.AllowList {
			if entry.Src == srcAssetID && entry.Dst == dstAssetID {
				newTask, err := f.enrichTaskBySrcDstAssets(task, srcAsset, dstAsset)
				return []ITask{newTask}, err
			}
		}
	}

	for _, pipeline := range f.AllPipelines {
		for _, entry := range pipeline.AllowList {
			if entry.Src == srcAssetID && entry.Dst == dstAssetID {
				result := []ITask{}
				for _, taskName := range pipeline.Tasks {
					task, err := f.findTask(taskName)
					if err != nil {
						return []ITask{}, fmt.Errorf("pipeline '%s' has invalid task: '%s'", pipeline.Name, taskName)
					}
					newTask, err := f.enrichTaskBySrcDstAssets(task, srcAsset, dstAsset)
					if err != nil {
						return []ITask{}, fmt.Errorf("pipeline '%s' can't enrich task: '%s' %s -> %s", pipeline.Name, taskName, srcAssetID, dstAssetID)
					}
					result = append(result, newTask)
				}
				return result, nil
			}
		}
	}

	return []ITask{}, fmt.Errorf("invalid path: '%s -> %s'", srcAssetID, dstAssetID)
}

func (f *Factory) cfgFromTaskByNameSrcDstAssetIDs(taskName string, srcAssetID AssetID, dstAssetID AssetID) (ITask, error) {
	// this function returns a task (instance) handling the two cases where the task comes from config or a pipeline

	// if there's no dst asset, then cfgFromTaskByNameSrcDstAssetIDs is identical to cfgFromTask
	if dstAssetID == "" {
		return f.cfgFromTask(taskName, srcAssetID)
	}

	// if a task is defined in the config, then again cfgFromTaskByNameSrcDstAssetIDs is identical to cfgFromTask
	task, err := f.cfgFromTask(taskName, srcAssetID)
	if err != nil {
		// continue
	} else {
		return task, nil
	}

	// if the task was not found in the config, attempt to load a pipeline by src/dst assets
	// then find the task by name within the pipeline
	srcAsset, _ := f.cfgFromAsset(srcAssetID)
	dstAsset, _ := f.cfgFromAsset(dstAssetID)
	pipeline, err := f.getTaskConfigBySrcDstAssets(srcAsset, dstAsset)
	if err != nil {
		return &TaskConfig{}, err
	}
	for _, task := range pipeline {
		if strings.EqualFold(string(task.ID()), taskName) {
			return task, nil
		}
	}
	return &TaskConfig{}, fmt.Errorf("invalid task: '%s' path: '%s -> %s'", taskName, srcAssetID, dstAssetID)
}

func (f *Factory) cfgFromMultiAsset(srcAssetID AssetID, dstAssetID AssetID) ([]ITask, error) {
	srcAsset, err := f.cfgFromAsset(srcAssetID)
	if err != nil {
		return []ITask{}, fmt.Errorf("invalid src asset in: '%s -> %s'", srcAssetID, dstAssetID)
	}
	if dstAssetID == "" {
		return []ITask{srcAsset}, err
	}
	dstAsset, err := f.cfgFromAsset(dstAssetID)
	if err != nil {
		return []ITask{}, fmt.Errorf("invalid dst asset in: '%s -> %s': %v", srcAssetID, dstAssetID, err)
	}

	return f.getTaskConfigBySrcDstAssets(srcAsset, dstAsset)
}

func (f *Factory) cfgEnrichToken(partialCfg *TokenAssetConfig) (*TokenAssetConfig, error) {
	cfg := partialCfg
	if cfg.Chain != "" {
		chainI, found := f.AllAssets.Load(AssetID(cfg.Chain))
		if !found {
			return cfg, fmt.Errorf("unsupported native asset: %s", cfg.Chain)
		}
		// make copy so edits do not persist to local store
		native := *chainI.(*ChainConfig)
		cfg.ChainConfig = &native
	} else {
		return cfg, fmt.Errorf("unsupported native asset: (empty)")
	}
	return cfg, nil
}

func (f *Factory) cfgEnrichDestinations(activity ITask, txInfo LegacyTxInfo) (LegacyTxInfo, error) {
	native := activity.GetChain()
	result := txInfo

	for i, dst := range txInfo.Destinations {
		dst.NativeAsset = NativeAsset(native.Chain)
		if dst.ContractAddress != "" {
			assetCfgI, err := f.cfgFromAssetByContract(string(dst.ContractAddress), dst.NativeAsset)
			if err != nil {
				// we shouldn't set the amount, if we don't know the contract
				continue
			}
			contractAddress := assetCfgI.GetContract()
			asset := assetCfgI.GetAssetSymbol()
			// this isn't really needed, but more to pass along a descriptive name
			dst.Asset = asset
			dst.ContractAddress = ContractAddress(contractAddress)
		}
		result.Destinations[i] = dst
	}
	return result, nil
}

// NewClient creates a new Client
func (f *Factory) NewClient(cfg ITask) (xclient.Client, error) {
	nativeAsset := cfg.GetChain()
	clients := nativeAsset.GetAllClients()
	if f.NoXcClients {
		// prevent recursion
		clients = nativeAsset.GetNativeClients()
	}
	for _, client := range clients {
		switch Driver(client.Driver) {
		case DriverCrosschain:
			return remoteclient.NewClient(cfg, client.Auth)
		default:
			return drivers.NewClient(cfg, Driver(client.Driver))
		}
	}
	return nil, fmt.Errorf("no clients possible for %s", nativeAsset.Chain)
}

func (f *Factory) NewStakingClient(stakingCfg *services.ServicesConfig, cfg ITask, provider StakingProvider) (xclient.StakingClient, error) {
	if !f.NoXcClients {
		clients := cfg.GetChain().GetAllClients()
		for _, client := range clients {
			switch Driver(client.Driver) {
			case DriverCrosschain:
				return remoteclient.NewStakingClient(cfg, client.Auth, stakingCfg.GetApiSecret(provider), provider)
			}
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
func (f *Factory) NewSigner(cfg ITask, secret string) (*signer.Signer, error) {
	return drivers.NewSigner(cfg, secret)
}

// NewAddressBuilder creates a new AddressBuilder
func (f *Factory) NewAddressBuilder(cfg ITask) (AddressBuilder, error) {
	return drivers.NewAddressBuilder(cfg)
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
	return getAddressFromPublicKey(cfg, publicKey)
}

// GetAllPossibleAddressesFromPublicKey returns all PossibleAddress(es) given a public key
func (f *Factory) GetAllPossibleAddressesFromPublicKey(cfg ITask, publicKey []byte) ([]PossibleAddress, error) {
	builder, err := drivers.NewAddressBuilder(cfg)
	if err != nil {
		return []PossibleAddress{}, err
	}
	return builder.GetAllPossibleAddressesFromPublicKey(publicKey)
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

// EnrichAssetConfig augments a partial AssetConfig, for example if some info is stored in a db and other in a config file
func (f *Factory) EnrichAssetConfig(partialCfg *TokenAssetConfig) (*TokenAssetConfig, error) {
	return f.cfgEnrichToken(partialCfg)
}

// EnrichDestinations augments a TxInfo by resolving assets and amounts in TxInfo.Destinations
func (f *Factory) EnrichDestinations(activity ITask, txInfo LegacyTxInfo) (LegacyTxInfo, error) {
	return f.cfgEnrichDestinations(activity, txInfo)
}

// GetAssetConfig returns an AssetConfig by asset and native asset (chain)
func (f *Factory) GetAssetConfig(asset string, nativeAsset NativeAsset) (ITask, error) {
	assetID := GetAssetIDFromAsset(asset, nativeAsset)
	return f.cfgFromAsset(assetID)
}

// GetTaskConfig returns an AssetConfig by task name and assetID
func (f *Factory) GetTaskConfig(taskName string, assetID AssetID) (ITask, error) {
	return f.cfgFromTask(taskName, assetID)
}

func (f *Factory) GetTaskConfigByNameSrcDstAssetIDs(taskName string, srcAssetID AssetID, dstAssetID AssetID) (ITask, error) {
	return f.cfgFromTaskByNameSrcDstAssetIDs(taskName, srcAssetID, dstAssetID)
}

// GetTaskConfigBySrcDstAssets returns an AssetConfig by source and destination assets
func (f *Factory) GetTaskConfigBySrcDstAssets(srcAsset ITask, dstAsset ITask) ([]ITask, error) {
	return f.getTaskConfigBySrcDstAssets(srcAsset, dstAsset)
}

func (f *Factory) RegisterGetAssetConfigCallback(callback func(assetID AssetID) (ITask, error)) {
	f.callbackGetAssetConfig = callback
}

func (f *Factory) UnregisterGetAssetConfigCallback() {
	f.callbackGetAssetConfig = nil
}

func (f *Factory) RegisterGetAssetConfigByContractCallback(callback func(contract string, nativeAsset NativeAsset) (ITask, error)) {
	f.callbackGetAssetConfigByContract = callback
}

func (f *Factory) UnregisterGetAssetConfigByContractCallback() {
	f.callbackGetAssetConfigByContract = nil
}

// GetMultiAssetConfig returns an AssetConfig by source and destination assetIDs
func (f *Factory) GetMultiAssetConfig(srcAssetID AssetID, dstAssetID AssetID) ([]ITask, error) {
	return f.cfgFromMultiAsset(srcAssetID, dstAssetID)
}

// GetAssetConfigByContract returns an AssetConfig by contract and native asset (chain)
func (f *Factory) GetAssetConfigByContract(contract string, nativeAsset NativeAsset) (ITask, error) {
	return f.cfgFromAssetByContract(contract, nativeAsset)
}

// PutAssetConfig adds an AssetConfig to the current Config cache
func (f *Factory) PutAssetConfig(cfgI ITask) (ITask, error) {
	f.AllAssets.Store(cfgI.ID(), cfgI)
	return f.cfgFromAsset(cfgI.ID())
}

// Config returns the Config
func (f *Factory) GetConfig() config.Config {
	return *f.Config
}

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

func getAddressFromPublicKey(cfg ITask, publicKey []byte) (Address, error) {
	builder, err := drivers.NewAddressBuilder(cfg)
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
