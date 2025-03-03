package sui_test

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
)

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := sui.NewAddressBuilder(xc.NewChainConfig("").Base())
	require.Nil(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := sui.NewAddressBuilder(xc.NewChainConfig("").Base())
	bytes, _ := hex.DecodeString("E0651D94176024B0C137C23A782D50291C04C8B5BCEDD4A7CD066BF4C0D21B22")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("0x086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8e"), address)

	bytes, _ = hex.DecodeString("00E0651D94176024B0C137C23A782D50291C04C8B5BCEDD4A7CD066BF4C0D21B22")
	address, err = builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("0x086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8e"), address)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKeyErr() {
	require := s.Require()
	builder, _ := sui.NewAddressBuilder(xc.NewChainConfig("").Base())

	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid length for ed25519 sui public key")

	address, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid length for ed25519 sui public key")

	bytes, _ := hex.DecodeString("01E0651D94176024B0C137C23A782D50291C04C8B5BCEDD4A7CD066BF4C0D21B22")
	address, err = builder.GetAddressFromPublicKey(bytes)
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid format for ed25519 sui public key")
}
