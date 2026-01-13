package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/near/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	pkBytes, err := hex.DecodeString("d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c")
	require.NoError(t, err)

	builder, err := address.NewAddressBuilder(xc.NewChainConfig(xc.NEAR).Base())
	require.NoError(t, err)

	address, err := builder.GetAddressFromPublicKey(pkBytes)
	require.Equal(t, xc.Address("d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c"), address)
	require.NoError(t, err)
}
