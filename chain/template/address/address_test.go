package address_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/template/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {

	builder, err := address.NewAddressBuilder(&xc.ChainConfig{})
	require.NotNil(t, builder)
	require.EqualError(t, err, "not implemented")
}

func TestGetAddressFromPublicKey(t *testing.T) {

	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	address, err := builder.GetAddressFromPublicKey([]byte{})
	require.Equal(t, xc.Address(""), address)
	require.EqualError(t, err, "not implemented")
}

func TestGetAllPossibleAddressesFromPublicKey(t *testing.T) {

	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey([]byte{})
	require.NotNil(t, addresses)
	require.EqualError(t, err, "not implemented")
}
