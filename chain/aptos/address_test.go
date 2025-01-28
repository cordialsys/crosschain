package aptos

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
)

func (s *AptosTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := NewAddressBuilder(&xc.ChainConfig{})
	require.Nil(err)
	require.NotNil(builder)
}

func (s *AptosTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("E0651D94176024B0C137C23A782D50291C04C8B5BCEDD4A7CD066BF4C0D21B22")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85"), address)

	bytes, _ = hex.DecodeString("E0651D94176024B0C137C23A782D50291C04C8B5BCEDD4A7CD066BF4C0D21B2200")
	address, err = builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85"), address)
}

func (s *AptosTestSuite) TestGetAddressFromPublicKeyErr() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})

	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid length for ed25519 public key")

	address, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid length for ed25519 public key")

	bytes, _ := hex.DecodeString("E0651D94176024B0C137C23A782D50291C04C8B5BCEDD4A7CD066BF4C0D21B2201")
	address, err = builder.GetAddressFromPublicKey(bytes)
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid format for ed25519 public key")
}
