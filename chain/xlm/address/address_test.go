package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(&xc.ChainConfig{})
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("a7451e1ff77aef667b7437098e7090fcaa24533cdbac626769bb6b5ec6b7d94c")
	addressFromPubKey, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4"), addressFromPubKey)
}

func TestGetAllPossibleAddressesFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})
	bytes, _ := hex.DecodeString("a7451e1ff77aef667b7437098e7090fcaa24533cdbac626769bb6b5ec6b7d94c")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, 1, len(addresses))
	require.Equal(t, xc.Address("GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4"), addresses[0].Address)
	require.Equal(t, xc.AddressTypeDefault, addresses[0].Type)
}
