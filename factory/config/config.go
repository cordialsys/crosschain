package config

import (
	"maps"
	"slices"
	"sort"

	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"
)

type NetworkSetting string

var Mainnet NetworkSetting = "mainnet"
var Testnet NetworkSetting = "testnet"

// Config is the full config containing all Assets
type Config struct {
	// which network to default to: "mainnet" or "testnet"
	// Default: "testnet"
	Network NetworkSetting `yaml:"network"`
	// map of lowercase(native_asset) -> NativeAssetObject
	Chains map[string]*xc.ChainConfig `yaml:"chains"`

	// Has this been parsed already
	parsed bool `yaml:"-"`
}

func (cfg *Config) MigrateFields() {
	for _, cfg := range cfg.Chains {
		if cfg.XAssetDeprecated != "" && cfg.Chain == "" {
			logrus.WithField("chain", cfg.Chain).Warn(".asset field is deprecated, please migrate to using .chain field instead")
			cfg.Chain = cfg.XAssetDeprecated
		}
	}
}

func (cfg *Config) Parse() {
	// Add all tokens + native assets to same list
	cfg.MigrateFields()
	for _, chain := range cfg.Chains {
		// migrate deprecated fields
		if chain.XAssetDeprecated != "" && chain.Chain == "" {
			chain.Chain = chain.XAssetDeprecated
		}
	}

	cfg.parsed = true
}

func (cfg *Config) GetChains() []*xc.ChainConfig {
	slice := slices.Collect(maps.Values(cfg.Chains))
	sort.Slice(slice, func(i, j int) bool {
		// need to be sorted deterministically
		return slice[i].ID() < slice[j].ID()
	})
	return slice
}
