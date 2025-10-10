package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/zcash/address"
	"github.com/stretchr/testify/require"
)

func TestAddressBuilder(t *testing.T) {
	require := require.New(t)
	chain := xc.NewChainConfig(xc.ZEC).WithNet("mainnet")
	builder, err := address.NewAddressBuilder(chain.Base())
	require.NoError(err)
	require.NotNil(builder)

	pubkey, err := hex.DecodeString("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
	require.NoError(err)

	address, err := builder.GetAddressFromPublicKey(pubkey)
	require.NoError(err)
	require.Equal(xc.Address("t1UYsZVJkLPeMjxEtACvSxfWuNmddpWfxzs"), address)
}
