package address_test

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/client"
	"github.com/cordialsys/crosschain/factory"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	require := require.New(t)

	builder, err := address.NewAddressBuilder(&xc.ChainConfig{ChainPrefix: "0"})
	require.Nil(err)
	require.NotNil(builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	require := require.New(t)
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{ChainPrefix: "0"})
	bytes, _ := hex.DecodeString("192c3c7e5789b461fbf1c7f614ba5eed0b22efc507cda60a5e7fda8e046bcdce")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4F"), address)
}

func TestGetAddressFromPublicKeyErr(t *testing.T) {
	require := require.New(t)
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{ChainPrefix: "0"})

	address, err := builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.ErrorContains(err, "invalid ed25519 public key")
}

func TestGetAllPossibleAddressesFromPublicKey(t *testing.T) {
	require := require.New(t)
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{ChainPrefix: "0"})
	bytes, _ := hex.DecodeString("192c3c7e5789b461fbf1c7f614ba5eed0b22efc507cda60a5e7fda8e046bcdce")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(1, len(addresses))
	require.Equal(xc.Address("1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4F"), addresses[0].Address)
	require.Equal(xc.AddressTypeDefault, addresses[0].Type)
}

func TestSubstrateChainsHavePrefix(t *testing.T) {
	require := require.New(t)

	configs := []*factory.Factory{
		factory.NewFactory(&factory.FactoryOptions{}),
		factory.NewNotMainnetsFactory(&factory.FactoryOptions{}),
	}
	for _, cfg := range configs {
		for _, asset := range cfg.GetAllAssets() {
			if chain, ok := asset.(*xc.ChainConfig); ok {
				if chain.Chain.Driver() == xc.DriverSubstrate {
					// validate that chain_prefix is set
					help := fmt.Sprintf("Invalid configuration for %s %s. Substrate chains must have the correct chain prefix u16 set, see https://polkadot.subscan.io/tools/format_transform",
						cfg.Config.Network,
						chain.Chain,
					)
					require.NotEmpty(chain.ChainPrefix, help)
					_, err := strconv.ParseUint(chain.ChainPrefix, 10, 16)
					require.NoError(err, help)

					// check indexer url
					if chain.IndexerType != client.IndexerRpc {
						help = fmt.Sprintf("Invalid configuration for %s %s. Need to use 'rpc' or set indexer_url for supported subscan or taostats endpoint, see https://support.subscan.io/",
							cfg.Config.Network,
							chain.Chain,
						)
						require.NotEmpty(chain.IndexerUrl, help)
					}
				}
			}
		}
	}
	builder, err := address.NewAddressBuilder(&xc.ChainConfig{ChainPrefix: "0"})
	require.Nil(err)
	require.NotNil(builder)
}
