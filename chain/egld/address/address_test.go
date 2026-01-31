package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/egld/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("EGLD").Base())
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("EGLD").Base())

	// Test vector: 32-byte public key
	// This will generate a valid bech32 address with "erd1" prefix
	pubKeyHex := "0139472eff6886771a982f3083da5d421f24c29181e63888228dc81ca60d69e1"
	pubKeyBytes, _ := hex.DecodeString(pubKeyHex)

	addr, err := builder.GetAddressFromPublicKey(pubKeyBytes)
	require.NoError(t, err)
	require.NotEmpty(t, addr)
	// Verify it starts with "erd1"
	require.Equal(t, "erd", string(addr)[:3])
}

func TestGetAddressFromPublicKeyErr(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("EGLD").Base())

	// Test with empty public key
	addr, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(t, xc.Address(""), addr)
	require.EqualError(t, err, "expected public key length 32, got 0")

	// Test with wrong length public key
	addr, err = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	require.Equal(t, xc.Address(""), addr)
	require.EqualError(t, err, "expected public key length 32, got 3")
}
