package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("").Base())
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("").Base())
	bytes, _ := hex.DecodeString("a7451e1ff77aef667b7437098e7090fcaa24533cdbac626769bb6b5ec6b7d94c")
	addressFromPubKey, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4"), addressFromPubKey)
}
