package setup

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
)

type ChainOverride struct {
	// The RPC URL for the chain
	Rpc string `json:"rpc,omitempty" toml:"rpc,omitempty"`
	// A secret that may be used with RPC access
	ApiKey string `json:"api_key,omitempty" toml:"api_key,omitempty"`
	// The network to use (e.g. mainnet/testnet/regtest on bitcoin chains)
	Network string `json:"network,omitempty" toml:"network,omitempty"`

	Applied bool `json:"-" toml:"-"`
}

func OverwriteCrosschainSettings(overrides map[string]*ChainOverride, xcFactory *factory.Factory) {
	if overrides == nil {
		return
	}
	for _, task := range xcFactory.GetAllAssets() {
		if chain, ok := task.(*xc.ChainConfig); ok {
			chainKey := strings.ToLower(string(chain.Chain))
			override, ok := overrides[chainKey]
			if ok {
				override.Applied = true
				if override.ApiKey != "" {
					logrus.WithField("chain", chain.Chain).Info("overriding api-key")
					chain.AuthSecret = override.ApiKey
				}
				if override.Rpc != "" {
					logrus.WithField("chain", chain.Chain).Info("overriding rpc")
					chain.URL = override.Rpc
					if strings.Contains(override.Rpc, "cordialapis.com") {
						logrus.WithField("chain", chain.Chain).WithField("rpc", chain.URL).Info("using cordialapis driver")
						// ensure crosschain driver is used
						chain.Driver = chain.Chain.Driver()
						chain.Clients = []*xc.ClientConfig{
							{
								Driver: xc.DriverCrosschain,
								URL:    chain.URL,
							},
						}
					} else {
						// ensure native driver is used
						chain.Clients = []*xc.ClientConfig{
							{
								Driver: chain.Chain.Driver(),
								URL:    chain.URL,
							},
						}
					}
				}
				if override.Network != "" {
					chain.Net = override.Network
				}
			}
		}
	}
	for chain, cfg := range overrides {
		if !cfg.Applied {
			logrus.WithField("chain", chain).Warn("could not find chain to apply override to")
		}
	}
}
