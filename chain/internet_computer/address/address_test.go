package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	pk := "9fd161cd0746135f2b97626c7f630f9726cc92547eb72725dd731bcf296130a0"
	pkBytes, err := hex.DecodeString(pk)
	require.NoError(t, err)

	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	address, err := builder.GetAddressFromPublicKey(pkBytes)
	require.Equal(t, xc.Address("5227b83cc14eda8a0ce76a7f2147071e60ee3502663b0efa4e10a4add469f107"), address)
	require.NoError(t, err)
}
