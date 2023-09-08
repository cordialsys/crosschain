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
	cfg := factoryconfig.Config{}
	if v := os.Getenv("XC_TESTNET"); v != "" {
		cfg.Network = "testnet"
	}

	err := config.RequireConfig("crosschain", &cfg, defaults.Mainnet)
	if err != nil {
		panic(err)
	}

	// use testnet
	switch cfg.Network {
	case "mainet":
		// done
	case "testnet":
		cfg = factoryconfig.Config{}
		err = config.RequireConfig("crosschain", &cfg, defaults.Testnet)
		if err != nil {
			panic(err)
		}
	default:
		// default to use mainnet to avoid using testnet by accident.
		cfg.Network = "mainnet"
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
		Config:       cfg,
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
				var err error
				native.AuthSecret, err = config.GetSecret(native.Auth)
				if err != nil {
					logrus.WithError(err).WithField("chain", native.GetChainIdentifier()).Error("could access secret")
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
