package defaults_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/factory"
	"github.com/stretchr/testify/require"
)

func TestDefaultChainConfigurationDriver(t *testing.T) {
	mainnetFactory := factory.NewDefaultFactory()
	testnetFactory := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	factories := []*factory.Factory{mainnetFactory, testnetFactory}
	for _, xcf := range factories {
		for _, asset := range xcf.GetAllAssets() {
			if chain, ok := asset.(*xc.ChainConfig); ok {
				switch chain.Driver {
				case xc.DriverEVM:
					err := evm.ValidateConfig(chain)
					require.NoError(t, err)
				default:
					// TODO provide config validation for all chains
				}
			}
		}
	}
}

func TestDefaultChainConfigurationLint(t *testing.T) {
	mainnetFactory := factory.NewDefaultFactory()
	testnetFactory := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	factories := []*factory.Factory{mainnetFactory, testnetFactory}
	for _, xcf := range factories {
		for _, asset := range xcf.GetAllAssets() {
			if chain, ok := asset.(*xc.ChainConfig); ok {
				require.NotEmpty(t, chain.Chain, "chain configuration entry with no chain set")
				require.NotEmpty(t, chain.Driver, fmt.Sprintf("chain %s must have driver set", chain.Chain))
				require.NotEmpty(t, chain.Driver.PublicKeyFormat(), fmt.Sprintf("chain %s driver '%s' is invalid", chain.Chain, chain.Driver))

				for _, provider := range chain.Staking.Providers {
					require.True(t, provider.Valid(), fmt.Sprintf("chain %s staking provider '%s' is invalid", chain.Chain, provider))
				}

			}
		}
	}
}
