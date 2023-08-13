package config

import (
	"strings"

	"github.com/jinzhu/copier"
	xc "github.com/jumpcrypto/crosschain"
)

// Config is the full config containing all Assets
type Config struct {
	// which network to default to: "mainnet" or "testnet"
	// Default: "testnet"

	Network   string                  `yaml:"network"`
	Chains    []*xc.NativeAssetConfig `yaml:"chains"`
	Tokens    []*xc.TokenAssetConfig  `yaml:"tokens"`
	Pipelines []*xc.PipelineConfig    `yaml:"pipelines"`

	Tasks []*xc.TaskConfig `yaml:"tasks"`

	chainsAndTokens []xc.ITask `yaml:"-"`
	// Has this been parsed already
	parsed bool `yaml:"-"`
}

func (cfg *Config) Parse() {
	// TODO remove AssetConfig object so we can remove this backwards
	// compat hack
	for _, t := range cfg.Tokens {
		copier.CopyWithOption(&t.AssetConfig, &t, copier.Option{IgnoreEmpty: false, DeepCopy: false})
	}
	// Add all tokens + native assets to same list
	cfg.chainsAndTokens = []xc.ITask{}
	for _, chain := range cfg.Chains {
		cfg.chainsAndTokens = append(cfg.chainsAndTokens, chain)
	}
	for _, token := range cfg.Tokens {
		cfg.chainsAndTokens = append(cfg.chainsAndTokens, token)
	}

	for _, task := range cfg.Tasks {
		task.AllowList = parseAllowList(task.Allow)
	}
	for _, pipeline := range cfg.Pipelines {
		pipeline.AllowList = parseAllowList(pipeline.Allow)
	}

	cfg.parsed = true
}

func (cfg *Config) GetChainsAndTokens() []xc.ITask {
	if !cfg.parsed {
		cfg.Parse()
	}
	return cfg.chainsAndTokens
}
func (cfg *Config) GetTasks() []*xc.TaskConfig {
	if !cfg.parsed {
		cfg.Parse()
	}
	return cfg.Tasks
}
func (cfg *Config) GetPipelines() []*xc.PipelineConfig {
	if !cfg.parsed {
		cfg.Parse()
	}
	return cfg.Pipelines
}

func parseAllowList(allowList []string) []*xc.AllowEntry {
	result := []*xc.AllowEntry{}
	for _, allow := range allowList {
		var entry xc.AllowEntry
		values := strings.Split(allow, "->")
		if len(values) == 1 {
			valueStr := strings.TrimSpace(values[0])
			value := xc.GetAssetIDFromAsset(valueStr, "")
			if valueStr == "*" {
				value = "*"
			}
			entry = xc.AllowEntry{
				Src: value,
				Dst: value,
			}
		}
		if len(values) == 2 {
			src := xc.GetAssetIDFromAsset(strings.TrimSpace(values[0]), "")
			dst := xc.GetAssetIDFromAsset(strings.TrimSpace(values[1]), "")
			entry = xc.AllowEntry{
				Src: src,
				Dst: dst,
			}
		}
		result = append(result, &entry)
	}
	return result
}
