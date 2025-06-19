package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/eos/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {

	builder, err := address.NewAddressBuilder(xc.NewChainConfig("EOS").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("EOS").Base())
	chksum := "2a3a2156"
	_ = chksum
	pubKey, err := hex.DecodeString("02e15a9a7aaf0c1e081a3b0f85f8fe12d53e2d19270fe21d0586c00213df2247ae")
	require.NoError(t, err)

	address, err := builder.GetAddressFromPublicKey(pubKey)
	require.NoError(t, err)
	require.Equal(t, xc.Address("EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s"), address)
}
