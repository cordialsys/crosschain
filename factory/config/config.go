package config

import (
	"sort"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"
)

// Config is the full config containing all Assets
type Config struct {
	// which network to default to: "mainnet" or "testnet"
	// Default: "testnet"

	Network string `yaml:"network"`
	// map of lowercase(native_asset) -> NativeAssetObject
	Chains map[string]*xc.NativeAssetConfig `yaml:"chains"`
	// map of lowercase(id) -> TokenAssetConfig
	Tokens map[string]*xc.TokenAssetConfig `yaml:"tokens"`

	// map of lowercase(id) -> TaskConfig
	Tasks map[string]*xc.TaskConfig `yaml:"tasks"`
	// map of lowercase(id) -> PipelineConfig
	Pipelines map[string]*xc.PipelineConfig `yaml:"pipelines"`

	chainsAndTokens []xc.ITask `yaml:"-"`
	// Has this been parsed already
	parsed bool `yaml:"-"`
}

func (cfg *Config) Parse() {
	// Add all tokens + native assets to same list
	cfg.chainsAndTokens = []xc.ITask{}
	for _, chain := range cfg.Chains {
		cfg.chainsAndTokens = append(cfg.chainsAndTokens, chain)
	}
	for _, token := range cfg.Tokens {
		// match to a chain
		for _, chain := range cfg.Chains {
			if chain.Asset == token.Chain {
				copy := *chain
				token.NativeAssetConfig = &copy
			}
		}
		// only add if matched to a chain
		if token.NativeAssetConfig != nil {
			cfg.chainsAndTokens = append(cfg.chainsAndTokens, token)
		} else {
			logrus.WithField("token", token).Warn("could not match token to a chain")
		}
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
	// must sort deterministically
	sort.Slice(cfg.chainsAndTokens, func(i, j int) bool {
		return cfg.chainsAndTokens[i].ID() < cfg.chainsAndTokens[j].ID()
	})
	return cfg.chainsAndTokens
}
func (cfg *Config) GetTasks() []*xc.TaskConfig {
	if !cfg.parsed {
		cfg.Parse()
	}
	return mapToList(cfg.Tasks)
}
func (cfg *Config) GetPipelines() []*xc.PipelineConfig {
	if !cfg.parsed {
		cfg.Parse()
	}
	return mapToList(cfg.Pipelines)
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

type HasID interface {
	ID() xc.AssetID
}

func mapToList[T HasID](theMap map[string]T) []T {
	toList := make([]T, len(theMap))
	i := 0
	for _, item := range theMap {
		toList[i] = item
		i++
	}
	sort.Slice(toList, func(i, j int) bool {
		// need to be sorted deterministically
		return toList[i].ID() < toList[j].ID()
	})
	return toList
}
