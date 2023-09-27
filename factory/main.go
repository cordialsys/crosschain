package factory

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"

	. "github.com/cordialsys/crosschain"
	xcclient "github.com/cordialsys/crosschain/chain/crosschain"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/drivers"
)

// FactoryContext is the main Factory interface
type FactoryContext interface {
	NewClient(asset ITask) (Client, error)
	NewTxBuilder(asset ITask) (TxBuilder, error)
	NewSigner(asset ITask) (Signer, error)
	NewAddressBuilder(asset ITask) (AddressBuilder, error)

	MarshalTxInput(input TxInput) ([]byte, error)
	UnmarshalTxInput(data []byte) (TxInput, error)

	GetAddressFromPublicKey(asset ITask, publicKey []byte) (Address, error)
	GetAllPossibleAddressesFromPublicKey(asset ITask, publicKey []byte) ([]PossibleAddress, error)

	MustAmountBlockchain(asset ITask, humanAmountStr string) AmountBlockchain
	MustAddress(asset ITask, addressStr string) Address
	MustPrivateKey(asset ITask, privateKey string) PrivateKey

	ConvertAmountToHuman(asset ITask, blockchainAmount AmountBlockchain) (AmountHumanReadable, error)
	ConvertAmountToBlockchain(asset ITask, humanAmount AmountHumanReadable) (AmountBlockchain, error)
	ConvertAmountStrToBlockchain(asset ITask, humanAmountStr string) (AmountBlockchain, error)

	EnrichAssetConfig(partialCfg *TokenAssetConfig) (*TokenAssetConfig, error)
	EnrichDestinations(asset ITask, txInfo TxInfo) (TxInfo, error)

	GetAssetConfig(asset string, nativeAsset string) (ITask, error)
	GetAssetConfigByContract(contract string, nativeAsset string) (ITask, error)
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
	RegisterGetAssetConfigByContractCallback(callback func(contract string, nativeAsset string) (ITask, error))
	UnregisterGetAssetConfigByContractCallback()
}

// Factory is the main Factory implementation, holding the config
type Factory struct {
	AllAssets                        *sync.Map
	AllTasks                         []*TaskConfig
	AllPipelines                     []*PipelineConfig
	callbackGetAssetConfig           func(assetID AssetID) (ITask, error)
	callbackGetAssetConfigByContract func(contract string, nativeAsset string) (ITask, error)
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
		asset_i := tasks[i].GetAssetConfig()
		asset_j := tasks[j].GetAssetConfig()
		key1 := asset_i.Asset + string(asset_i.NativeAsset) + asset_i.Chain
		key2 := asset_j.Asset + string(asset_j.NativeAsset) + asset_j.Chain
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
		return &NativeAssetConfig{}, fmt.Errorf("could not lookup asset: '%s'", assetID)
	}
	if cfg, ok := cfgI.(*NativeAssetConfig); ok {
		// native asset
		cfg.Type = AssetTypeNative
		cfg.Chain = cfg.Asset
		cfg.NativeAsset = NativeAsset(cfg.Asset)
		return cfg, nil
	}
	if cfg, ok := cfgI.(*TokenAssetConfig); ok {
		// token
		copier.CopyWithOption(&cfg.AssetConfig, &cfg, copier.Option{IgnoreEmpty: false, DeepCopy: false})
		cfg, _ = f.cfgEnrichAssetConfig(cfg)
		return cfg, nil
	}
	return &NativeAssetConfig{}, fmt.Errorf("invalid asset: '%s'", assetID)
}

func (f *Factory) cfgFromAssetByContract(contract string, nativeAsset string) (ITask, error) {
	var res ITask
	contract = NormalizeAddressString(contract, nativeAsset)
	f.AllAssets.Range(func(key, value interface{}) bool {
		cfg := value.(ITask).GetAssetConfig()
		if cfg.Chain == nativeAsset {
			cfgContract := NormalizeAddressString(cfg.Contract, nativeAsset)
			if cfgContract == contract {
				res = value.(ITask)
				return false
			}
		} else if cfg.Asset == nativeAsset && cfg.ChainCoin == contract {
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
	return &TokenAssetConfig{}, fmt.Errorf("invalid contract: '%s'", contract)
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

func (f *Factory) cfgEnrichAssetConfig(partialCfg *TokenAssetConfig) (*TokenAssetConfig, error) {
	cfg := partialCfg
	if cfg.Chain != "" {
		// token
		if cfg.Type == "" {
			cfg.Type = AssetTypeToken
		}
		if cfg.AssetConfig.Type == "" {
			cfg.AssetConfig.Type = AssetTypeToken
		}
		nativeAsset := cfg.Chain
		cfg.NativeAsset = NativeAsset(nativeAsset)

		chainI, found := f.AllAssets.Load(AssetID(nativeAsset))
		if !found {
			return cfg, fmt.Errorf("unsupported native asset: %s", nativeAsset)
		}
		chain := chainI.(*NativeAssetConfig)
		cfg.NativeAssetConfig = chain
		if cfg.NativeAssetConfig.NativeAsset == "" {
			cfg.NativeAssetConfig.NativeAsset = NativeAsset(cfg.NativeAssetConfig.Asset)
		}
		// deprecated fields below
		cfg.Driver = chain.Driver
		cfg.Net = chain.Net
		cfg.URL = chain.URL
		cfg.FcdURL = chain.FcdURL
		cfg.Auth = chain.Auth
		cfg.AuthSecret = chain.AuthSecret
		cfg.Provider = chain.Provider
		cfg.ChainID = chain.ChainID
		cfg.ChainIDStr = chain.ChainIDStr
		cfg.ChainGasMultiplier = chain.ChainGasMultiplier
		cfg.ExplorerURL = chain.ExplorerURL
		cfg.NoGasFees = chain.NoGasFees
		cfg.GasCoin = chain.GasCoin
		cfg.ChainPrefix = chain.ChainPrefix
	} else {
		return cfg, fmt.Errorf("unsupported native asset: (empty)")
	}
	return cfg, nil
}

func (f *Factory) cfgEnrichDestinations(activity ITask, txInfo TxInfo) (TxInfo, error) {
	asset := activity.GetAssetConfig()
	result := txInfo
	nativeAssetCfg := activity.GetNativeAsset()
	for i, dst := range txInfo.Destinations {
		dst.NativeAsset = asset.NativeAsset
		if dst.ContractAddress != "" {
			assetCfgI, err := f.cfgFromAssetByContract(string(dst.ContractAddress), string(dst.NativeAsset))
			if err != nil {
				// we shouldn't set the amount, if we don't know the contract
				continue
			}
			assetCfg := assetCfgI.GetAssetConfig()
			dst.Asset = Asset(assetCfg.Asset)
			dst.ContractAddress = ContractAddress(assetCfg.Contract)
			dst.AssetConfig = assetCfg
		} else {
			dst.AssetConfig = nativeAssetCfg
		}
		result.Destinations[i] = dst
	}
	return result, nil
}

// NewClient creates a new Client
func (f *Factory) NewClient(cfg ITask) (Client, error) {
	nativeAsset := cfg.GetNativeAsset()
	clients := nativeAsset.GetAllClients()
	if f.NoXcClients {
		// prevent recursion
		clients = nativeAsset.GetNativeClients()
	}
	for _, client := range clients {
		switch Driver(client.Driver) {
		case DriverCrosschain:
			return xcclient.NewClient(cfg)
		default:
			return drivers.NewClient(cfg, Driver(client.Driver))
		}
	}
	return nil, errors.New("no clients possible for " + nativeAsset.Asset)
}

// NewTxBuilder creates a new TxBuilder
func (f *Factory) NewTxBuilder(cfg ITask) (TxBuilder, error) {
	return drivers.NewTxBuilder(cfg)
}

// NewSigner creates a new Signer
func (f *Factory) NewSigner(cfg ITask) (Signer, error) {
	return drivers.NewSigner(cfg)
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
	return blockchainAmount.ToHuman(cfg.GetAssetConfig().Decimals), nil
}

// ConvertAmountToBlockchain converts an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *Factory) ConvertAmountToBlockchain(cfg ITask, humanAmount AmountHumanReadable) (AmountBlockchain, error) {
	return humanAmount.ToBlockchain(cfg.GetAssetConfig().Decimals), nil
}

// ConvertAmountStrToBlockchain converts a string representing an AmountHumanReadable into AmountBlockchain, multiplying by the appropriate number of decimals
func (f *Factory) ConvertAmountStrToBlockchain(cfg ITask, humanAmountStr string) (AmountBlockchain, error) {
	_, err := decimal.NewFromString(humanAmountStr)
	return NewAmountHumanReadableFromStr(humanAmountStr).
		ToBlockchain(cfg.GetAssetConfig().Decimals), err
}

// EnrichAssetConfig augments a partial AssetConfig, for example if some info is stored in a db and other in a config file
func (f *Factory) EnrichAssetConfig(partialCfg *TokenAssetConfig) (*TokenAssetConfig, error) {
	return f.cfgEnrichAssetConfig(partialCfg)
}

// EnrichDestinations augments a TxInfo by resolving assets and amounts in TxInfo.Destinations
func (f *Factory) EnrichDestinations(activity ITask, txInfo TxInfo) (TxInfo, error) {
	return f.cfgEnrichDestinations(activity, txInfo)
}

// GetAssetConfig returns an AssetConfig by asset and native asset (chain)
func (f *Factory) GetAssetConfig(asset string, nativeAsset string) (ITask, error) {
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

func (f *Factory) RegisterGetAssetConfigByContractCallback(callback func(contract string, nativeAsset string) (ITask, error)) {
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
func (f *Factory) GetAssetConfigByContract(contract string, nativeAsset string) (ITask, error) {
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

// MustPrivateKey coverts a string into PrivateKey, panic if error
func (f *Factory) MustPrivateKey(cfg ITask, privateKeyStr string) PrivateKey {
	signer, err := f.NewSigner(cfg)
	if err != nil {
		panic(err)
	}
	privateKey, err := signer.ImportPrivateKey(privateKeyStr)
	if err != nil {
		panic(err)
	}
	return privateKey
}

func getAddressFromPublicKey(cfg ITask, publicKey []byte) (Address, error) {
	builder, err := drivers.NewAddressBuilder(cfg)
	if err != nil {
		return "", err
	}
	return builder.GetAddressFromPublicKey(publicKey)
}

func CheckError(driver Driver, err error) ClientError {
	return drivers.CheckError(driver, err)
}
