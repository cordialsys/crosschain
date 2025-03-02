package config

import (
	"maps"
	"slices"
	"sort"

	xc "github.com/cordialsys/crosschain"
)

type NetworkSetting string

var Mainnet NetworkSetting = "mainnet"
var Testnet NetworkSetting = "testnet"
var NotMainnet NetworkSetting = "!mainnet"

func (setting NetworkSetting) Selector() xc.NetworkSelector {
	if setting == Testnet {
		return xc.NotMainnets
	}
	if setting == NotMainnet {
		return xc.NotMainnets
	}
	return xc.Mainnets
}

// Config is the full config containing all Assets
type Config struct {
	// which network to default to: "mainnet" or "testnet"
	// Default: "testnet"
	Network       NetworkSetting `yaml:"network"`
	CrosschainUrl string         `yaml:"crosschain_url"`

	// map of lowercase(native_asset) -> NativeAssetObject
	Chains map[string]*xc.ChainConfig `yaml:"chains"`

	// Has this been parsed already
	parsed bool `yaml:"-"`
}

func (cfg *Config) MigrateFields() {
	for _, cfg := range cfg.Chains {
		if cfg.ChainBaseConfig == nil {
			cfg.ChainBaseConfig = &xc.ChainBaseConfig{}
		}
		if cfg.ChainClientConfig == nil {
			cfg.ChainClientConfig = &xc.ChainClientConfig{}
		}
		cfg.Configure()
	}
}

func (cfg *Config) Parse() {
	// Add all tokens + native assets to same list
	cfg.MigrateFields()

	cfg.parsed = true
}

func (cfg *Config) GetChains() []*xc.ChainConfig {
	slice := slices.Collect(maps.Values(cfg.Chains))
	sort.Slice(slice, func(i, j int) bool {
		// need to be sorted deterministically
		return slice[i].Chain < slice[j].Chain
	})
	return slice
}
