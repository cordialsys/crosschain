package setup

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
)

type ChainOverride struct {
	// The RPC URL for the chain
	Rpc string `json:"rpc,omitempty" toml:"rpc,omitempty"`
	// A secret reference that may be used with RPC access
	ApiKey config.Secret `json:"api_key,omitempty" toml:"api_key,omitempty"`
	// The network to use (e.g. mainnet/testnet/regtest on bitcoin chains)
	Network string `json:"network,omitempty" toml:"network,omitempty"`

	Applied bool `json:"-" toml:"-"`
}

func OverwriteCrosschainSettings(overrides map[string]*ChainOverride, xcFactory *factory.Factory) {
	if overrides == nil {
		return
	}
	for _, chain := range xcFactory.GetAllChains() {
		chainKey := strings.ToLower(string(chain.Chain))
		override, ok := overrides[chainKey]
		if ok {
			override.Applied = true
			if override.ApiKey != "" {
				logrus.WithField("chain", chain.Chain).Info("overriding api-key")
				chain.Auth2 = override.ApiKey
			}
			if override.Rpc != "" {
				logrus.WithField("chain", chain.Chain).Info("overriding rpc")
				chain.URL = override.Rpc
				if strings.Contains(override.Rpc, "cordialapis.com") {
					logrus.WithField("chain", chain.Chain).WithField("rpc", chain.URL).Info("using cordialapis driver")
					// ensure crosschain driver is used
					chain.Driver = xc.DriverCrosschain
				}
			}
			if override.Network != "" {
				chain.Network = override.Network
			}
		}
	}
	for chain, cfg := range overrides {
		if !cfg.Applied {
			logrus.WithField("chain", chain).Warn("could not find chain to apply override to")
		}
	}
}
