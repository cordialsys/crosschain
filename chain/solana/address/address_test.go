package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana/address"
	"github.com/test-go/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(&xc.ChainConfig{})
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("FC880863219008406235FA4C8FBB2A86D3DA7B6762EAC39323B2A1D8C404A414")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb"), address)
}

func TestGetAddressFromPublicKeyErr(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})

	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(t, xc.Address(""), address)
	require.EqualError(t, err, "expected address length 32, got address length 0")

	address, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(t, xc.Address(""), address)
	require.EqualError(t, err, "expected address length 32, got address length 3")
}
