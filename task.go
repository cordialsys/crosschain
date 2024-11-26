package crosschain

import "fmt"

// Task represents a tx, e.g. smart contract function call, on a blockchain.
type Task string

// TaskConfig is the model used to represent a task read from config file or db
type TaskConfig struct {
	Name          string                 `yaml:"name"`
	Code          string                 `yaml:"code"`
	Allow         []string               `yaml:"allow"`
	Chain         string                 `yaml:"chain"`
	DefaultParams map[string]interface{} `yaml:"default_params"`
	Operations    []TaskConfigOperation  `yaml:"operations"`

	// internal
	AllowList []*AllowEntry `yaml:"-"`
	SrcAsset  ITask         `yaml:"-"`
	DstAsset  ITask         `yaml:"-"`
}

var _ ITask = &TaskConfig{}

// PipelineConfig is the model used to represent a pipeline (list of tasks) read from config file or db
type PipelineConfig struct {
	Name  string   `yaml:"name"`
	Allow []string `yaml:"allow"`
	Tasks []string `yaml:"tasks"`

	// internal
	AllowList []*AllowEntry `yaml:"-"`
}

func (p PipelineConfig) String() string {
	return fmt.Sprintf(
		"PipelineConfig(id=%s)",
		p.Name,
	)
}

func (p PipelineConfig) ID() AssetID {
	return AssetID(p.Name)
}

type AllowEntry struct {
	Src AssetID
	Dst AssetID
}

type TaskConfigOperation struct {
	Function  string                     `yaml:"function"`
	Signature string                     `yaml:"signature"`
	Contract  interface{}                `yaml:"contract"` // string or map[string]string
	Payable   bool                       `yaml:"payable"`
	Params    []TaskConfigOperationParam `yaml:"params"`
}

type TaskConfigOperationParam struct {
	Name  string      `yaml:"name"`
	Type  string      `yaml:"type"`
	Bind  string      `yaml:"bind"`
	Match string      `yaml:"match"`
	Value interface{} `yaml:"value"` // string or map[string]string
	// Defaults []TaskConfigOperationParamDefaults `yaml:"defaults"`
	Fields []TaskConfigOperationParam `yaml:"fields"`
}

type TaskConfigOperationParamDefaults struct {
	Match string `yaml:"match"`
	Value string `yaml:"value"`
}

type ITask interface {
	// unique identifier used within crosschain, typically a combination of asset.chain
	ID() AssetID
	GetChain() *ChainConfig
	// Get associated asset decimals if it exists
	GetDecimals() int32
	// Get associated contract if it exists
	GetContract() string

	// Informational / debugging
	String() string
	// Get asset symbol (e.g. 'USDC') if it exists.  Does not include the chain.  Informational.
	GetAssetSymbol() string
}

func (task TaskConfig) String() string {
	src := "not-set"
	if task.SrcAsset != nil {
		src = string(task.SrcAsset.ID())
	}
	dst := "not-set"
	if task.DstAsset != nil {
		dst = string(task.DstAsset.ID())
	}
	return fmt.Sprintf(
		"TaskConfig(id=%s src=%s dst=%s defaults=%v)",
		task.ID(), src, dst, task.DefaultParams,
	)
}

func (task *TaskConfig) ID() AssetID {
	return AssetID(task.Name)
}

func (task *TaskConfig) GetDecimals() int32 {
	// source asset is the asset being used typically
	return task.SrcAsset.GetDecimals()
}

func (task *TaskConfig) GetContract() string {
	// by default we return the source asset contract
	return task.SrcAsset.GetContract()
}

func (task *TaskConfig) GetAssetSymbol() string {
	// by default we return the source asset contract
	return task.SrcAsset.GetAssetSymbol()
}

func (task TaskConfig) GetChain() *ChainConfig {
	return task.SrcAsset.GetChain()
}

func (task TaskConfig) GetTask() *TaskConfig {
	return &task
}

func (task *TaskConfig) IsChain(contract ContractAddress) bool {
	return contract == "" || task.GetChain().Chain == NativeAsset(contract)
}
