package address_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/template/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {

	builder, err := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	require.NotNil(t, builder)
	require.EqualError(t, err, "not implemented")
}

func TestGetAddressFromPublicKey(t *testing.T) {

	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(t, xc.Address(""), address)
	require.EqualError(t, err, "not implemented")
}
