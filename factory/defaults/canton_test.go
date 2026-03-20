package defaults_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/stretchr/testify/require"
)

func TestCantonConfigTestnet(t *testing.T) {
	// Load testnet factory
	testnetFactory := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	require.NotNil(t, testnetFactory)

	// Get Canton configuration
	config, found := testnetFactory.GetChain(xc.CANTON)
	require.True(t, found, "Canton should be configured in testnet")
	require.NotNil(t, config)

	// Verify basic configuration
	require.Equal(t, xc.CANTON, config.Chain)
	require.Equal(t, xc.DriverCanton, config.Driver)
	require.Equal(t, "Canton (Testnet)", config.ChainName)
	require.Equal(t, int32(18), config.Decimals)
}

func TestCantonConfigMainnet(t *testing.T) {
	// Load mainnet factory
	mainnetFactory := factory.NewDefaultFactory()
	require.NotNil(t, mainnetFactory)

	// Get Canton configuration
	config, found := mainnetFactory.GetChain(xc.CANTON)
	require.True(t, found, "Canton should be configured in mainnet")
	require.NotNil(t, config)

	// Verify basic configuration
	require.Equal(t, xc.CANTON, config.Chain)
	require.Equal(t, xc.DriverCanton, config.Driver)
	require.Equal(t, "Canton", config.ChainName)
	require.Equal(t, int32(18), config.Decimals)
}
