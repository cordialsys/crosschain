package solana

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
)

func (s *SolanaTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := NewAddressBuilder(&xc.ChainConfig{})
	require.Nil(err)
	require.NotNil(builder)
}

func (s *SolanaTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("FC880863219008406235FA4C8FBB2A86D3DA7B6762EAC39323B2A1D8C404A414")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb"), address)
}

func (s *SolanaTestSuite) TestGetAddressFromPublicKeyErr() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})

	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "expected address length 32, got address length 0")

	address, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "expected address length 32, got address length 3")
}

func (s *SolanaTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("FC880863219008406235FA4C8FBB2A86D3DA7B6762EAC39323B2A1D8C404A414")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(1, len(addresses))
	require.Equal(xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb"), addresses[0].Address)
	require.Equal(xc.AddressTypeDefault, addresses[0].Type)
}
