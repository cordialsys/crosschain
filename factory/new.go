package factory

import (
	"os"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	factoryconfig "github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults"
)

type FactoryOptions struct {
	// do not use chains that have been marked disabled
	UseDisabledChains bool
	// do not use xc clients, only use native clients
	NoXcClients bool
}

func NewFactory(options *FactoryOptions) *Factory {
	// Use our config file loader
	// Load into an empty config just to detect if the user passed in a .Network value.
	detectMainnetInConfig := factoryconfig.Config{}
	err := config.RequireConfig("crosschain", &detectMainnetInConfig, factoryconfig.Config{})
	if err != nil {
		panic(err)
	}

	cfg := factoryconfig.Config{}
	err = config.RequireConfig("crosschain", &cfg, defaults.Mainnet)
	if err != nil {
		panic(err)
	}

	if detectMainnetInConfig.Network == "" {
		// If the user did not pass in a network value, we will consider XC_TESTNET variable.
		if v := strings.ToLower(os.Getenv("XC_TESTNET")); v == "1" || v == "true" {
			cfg.Network = factoryconfig.Testnet
		}
	}

	switch cfg.Network {
	case "mainnet":
		// done
	case "testnet":
		cfg = factoryconfig.Config{Network: cfg.Network}
		err = config.RequireConfig("crosschain", &cfg, defaults.Testnet)
		if err != nil {
			panic(err)
		}
	default:
		// default to use mainnet to avoid using testnet by accident.
		cfg.Network = factoryconfig.Mainnet
	}

	return NewDefaultFactoryWithConfig(&cfg, options)
}

func NewNotMainnetsFactory(options *FactoryOptions) *Factory {
	cfg := factoryconfig.Config{}
	err := config.RequireConfig("crosschain", &cfg, defaults.Testnet)
	if err != nil {
		panic(err)
	}

	cfg.Network = factoryconfig.Testnet
	return NewDefaultFactoryWithConfig(&cfg, options)
}

// NewDefaultFactory creates a new Factory
func NewDefaultFactory() *Factory {
	return NewFactory(&FactoryOptions{
		UseDisabledChains: false,
	})
}

// NewDefaultFactoryWithConfig creates a new Factory given a config map
func NewDefaultFactoryWithConfig(cfg *factoryconfig.Config, options *FactoryOptions) *Factory {
	chainsList := cfg.GetChains()
	if options == nil {
		options = &FactoryOptions{}
	}

	factory := &Factory{
		AllChains:   []*xc.ChainConfig{},
		NoXcClients: options.NoXcClients,
		Config:      cfg,
	}
	for _, chain := range chainsList {
		disabled := chain.Disabled
		if disabled != nil && *disabled {
			// skip unless explicity including
			if !options.UseDisabledChains {
				continue
			}
		}
		factory.AllChains = append(factory.AllChains, chain)
		chain.Configure()
	}

	return factory
}
