package factory

import (
	"os"
	"sync"

	xc "github.com/jumpcrypto/crosschain"
	"github.com/jumpcrypto/crosschain/config"
	factoryconfig "github.com/jumpcrypto/crosschain/factory/config"
	"github.com/jumpcrypto/crosschain/factory/defaults"
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
	var cfg factoryconfig.Config
	if v := os.Getenv("XC_MAINNET"); v != "" {
		// use mainnets
		cfg = factoryconfig.Config{}
		err := config.RequireConfig("crosschain", &cfg, defaults.Mainnet)
		if err != nil {
			panic(err)
		}
	} else {
		// default to use testnet
		cfg = factoryconfig.Config{}
		err := config.RequireConfig("crosschain", &cfg, defaults.Testnet)
		if err != nil {
			panic(err)
		}

		// special override: if override with mainnet, let's start over with mainnet defaults
		if cfg.Network == "mainnet" {
			cfg = factoryconfig.Config{}
			err = config.RequireConfig("crosschain", &cfg, defaults.Mainnet)
			if err != nil {
				panic(err)
			}
		}
	}

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
	}
	for _, asset := range assetsList {
		disabled := asset.GetNativeAsset().Disabled
		if disabled != nil && *disabled {
			// skip unless explicity including
			if !options.UseDisabledChains {
				continue
			}
		}
		// dereference secrets
		if native, ok := asset.(*xc.NativeAssetConfig); ok {
			if native.Auth != "" {
				native.AuthSecret, _ = config.GetSecret(native.Auth)
			}
		}
		_, err := factory.PutAssetConfig(asset)
		if err != nil {
			logrus.WithError(err).WithField("asset", asset).Warn("could not add asset")
		}
	}

	return factory
}
