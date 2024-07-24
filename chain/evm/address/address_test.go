package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(&xc.ChainConfig{})
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("04760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1b01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("0x5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1"), address)

	bytes_compressed, _ := hex.DecodeString("0229f11138ff637ecef0d1878fb5aff4175e96c0758f2f32c004c8e9791e8750ab")
	address, err = builder.GetAddressFromPublicKey(bytes_compressed)
	require.NoError(t, err)
	require.Equal(t, xc.Address("0xCc10cd3f77d370F7893E94e4eEb48Fb9553B7a5B"), address)
}

func TestGetAddressFromPublicKeyErr(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})

	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(t, xc.Address(""), address)
	require.EqualError(t, err, "invalid secp256k1 public key")

	address, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(t, xc.Address(""), address)
	require.EqualError(t, err, "invalid secp256k1 public key")
}

func TestGetAllPossibleAddressesFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("04760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1b01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, 1, len(addresses))
	require.Equal(t, xc.Address("0x5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1"), addresses[0].Address)
	require.Equal(t, xc.AddressTypeDefault, addresses[0].Type)
}

func TestHexToAddress(t *testing.T) {
	addr, err := address.FromHex("0x5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1")
	require.NoError(t, err)
	require.Equal(t, common.Address{0x58, 0x91, 0x90, 0x6f, 0xEf, 0x64, 0xA5, 0xae, 0x92, 0x4C, 0x7F, 0xc5, 0xed, 0x48, 0xc0, 0xF6, 0x4a, 0x55, 0xfC, 0xe1}, addr)

	// common.HexToAddress adds a 0 if the size is not even
	addr, err = address.FromHex("0x891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1")
	require.NoError(t, err)
	require.Equal(t, common.Address{0x8, 0x91, 0x90, 0x6f, 0xEf, 0x64, 0xA5, 0xae, 0x92, 0x4C, 0x7F, 0xc5, 0xed, 0x48, 0xc0, 0xF6, 0x4a, 0x55, 0xfC, 0xe1}, addr)

	// xdc instead of 0x
	addr, err = address.FromHex("xdc5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1")
	require.NoError(t, err)
	require.Equal(t, common.Address{0x58, 0x91, 0x90, 0x6f, 0xEf, 0x64, 0xA5, 0xae, 0x92, 0x4C, 0x7F, 0xc5, 0xed, 0x48, 0xc0, 0xF6, 0x4a, 0x55, 0xfC, 0xe1}, addr)

	// this should probably never happen in practise, but just to test the implementation
	addr, err = address.FromHex("0xxdc5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1")
	require.NoError(t, err)
	require.Equal(t, common.Address{0x58, 0x91, 0x90, 0x6f, 0xEf, 0x64, 0xA5, 0xae, 0x92, 0x4C, 0x7F, 0xc5, 0xed, 0x48, 0xc0, 0xF6, 0x4a, 0x55, 0xfC, 0xe1}, addr)
}
