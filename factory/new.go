package factory

import (
	"os"
	"strings"
	"sync"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	factoryconfig "github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults"
	"github.com/sirupsen/logrus"
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
	assetsList := cfg.GetChainsAndTokens()
	if options == nil {
		options = &FactoryOptions{}
	}

	factory := &Factory{
		AllAssets:    &sync.Map{},
		AllTasks:     cfg.GetTasks(),
		AllPipelines: cfg.GetPipelines(),
		NoXcClients:  options.NoXcClients,
		Config:       cfg,
	}
	for _, asset := range assetsList {
		disabled := asset.GetChain().Disabled
		if disabled != nil && *disabled {
			// skip unless explicity including
			if !options.UseDisabledChains {
				continue
			}
		}
		// dereference secrets
		if native, ok := asset.(*xc.ChainConfig); ok {
			if native.Auth != "" {
				var err error
				native.AuthSecret, err = config.GetSecret(native.Auth)
				if err != nil {
					logrus.WithError(err).WithField("chain", native.Chain).Error("could not access secret")
				}
			}
		}
		_, err := factory.PutAssetConfig(asset)
		if err != nil {
			logrus.WithError(err).WithField("asset", asset).Warn("could not add asset")
		}
	}

	return factory
}
