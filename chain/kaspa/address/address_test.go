package address_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/kaspa/address"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("KAS").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetAddressFromPublicKey(t *testing.T) {

	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("XYZ").WithChainPrefix("kaspa").Base())
	publicKey := testutil.FromHex("e9f9c7e9b9c91544daeabf9a4c7d978318d5d914b14c2d6fb3741bfc81c94134")
	addr, err := builder.GetAddressFromPublicKey(publicKey)
	require.NoError(t, err)
	require.EqualValues(t, "kaspa:qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9qng8tfmwmf8", addr)

	builder, _ = address.NewAddressBuilder(xc.NewChainConfig("XYZ").WithChainPrefix("kaspadev").Base())
	publicKey = testutil.FromHex("e9f9c7e9b9c91544daeabf9a4c7d978318d5d914b14c2d6fb3741bfc81c94134")
	addr, err = builder.GetAddressFromPublicKey(publicKey)
	require.NoError(t, err)
	require.EqualValues(t, "kaspadev:qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9qngt4vauxut", addr)
}
