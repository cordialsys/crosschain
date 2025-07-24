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
	pk := "1d25811b76f43c86d59d757622773b2969ee71270ea810a42deda024e0cf896a"
	pkBytes, err := hex.DecodeString(pk)
	require.NoError(t, err)

	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	address, err := builder.GetAddressFromPublicKey(pkBytes)
	require.Equal(t, xc.Address("d10faafe5dbce0649eeaf68cab767602ec39795c2b5eabf51da063f9de5d1464"), address)
	require.NoError(t, err)
}
