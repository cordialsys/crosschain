package substrate

import (
	"encoding/hex"

	xc "github.com/jumpcrypto/crosschain"
)

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := NewAddressBuilder(&xc.AssetConfig{})
	require.Nil(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{})
	bytes, _ := hex.DecodeString("192c3c7e5789b461fbf1c7f614ba5eed0b22efc507cda60a5e7fda8e046bcdce")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4F"), address)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKeyErr() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{})

	address, err := builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid sr25519 public key")
}

func (s *CrosschainTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{})
	bytes, _ := hex.DecodeString("192c3c7e5789b461fbf1c7f614ba5eed0b22efc507cda60a5e7fda8e046bcdce")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(1, len(addresses))
	require.Equal(xc.Address("1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4F"), addresses[0].Address)
	require.Equal(xc.AddressTypeDefault, addresses[0].Type)
}
