package tron

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := NewAddressBuilder(&xc.ChainConfig{})
	require.NoError(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("0404B604296010A55D40000B798EE8454ECCC1F8900E70B1ADF47C9887625D8BAE3866351A6FA0B5370623268410D33D345F63344121455849C9C28F9389ED9731")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(err)
	require.Equal(xc.Address("TDpBe64DqirkKWj6HWuR1pWgmnhw2wDacE"), address)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKeyErr() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.ChainConfig{})

	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid secp256k1 public key")

	address, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "invalid secp256k1 public key")
}

// func (s *CrosschainTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
// 	require := s.Require()
// 	builder, _ := NewAddressBuilder(&xc.ChainConfig{})
// 	bytes, _ := hex.DecodeString("04760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1b01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d")
// 	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
// 	require.Nil(err)
// 	require.Equal(1, len(addresses))
// 	require.Equal(xc.Address("0x5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1"), addresses[0].Address)
// 	require.Equal(xc.AddressTypeDefault, addresses[0].Type)
// }
