package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	require := require.New(t)

	builder, err := address.NewAddressBuilder(xc.NewChainConfig("").WithChainPrefix("0").Base())
	require.Nil(err)
	require.NotNil(builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	require := require.New(t)
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("").WithChainPrefix("0").Base())
	bytes, _ := hex.DecodeString("192c3c7e5789b461fbf1c7f614ba5eed0b22efc507cda60a5e7fda8e046bcdce")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4F"), address)
}

func TestGetAddressFromPublicKeyErr(t *testing.T) {
	require := require.New(t)
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("").WithChainPrefix("0").Base())

	address, err := builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.ErrorContains(err, "invalid ed25519 public key")
}
