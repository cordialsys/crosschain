package address_test

import (
	"encoding/hex"
	"testing"

	"github.com/cloudflare/circl/sign/bls"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/dusk/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("DUSK").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	pk := []byte{143, 218, 164, 37, 33, 79, 171, 35, 173, 65, 172, 42, 108, 25, 178, 74, 110, 127, 10, 67, 124, 101, 104, 2, 67, 196, 60, 34, 21, 251, 26, 229, 129, 213, 53, 107, 156, 63, 232, 139, 18, 72, 106, 77, 164, 252, 24, 120, 7, 142, 82, 63, 151, 100, 234, 175, 108, 8, 96, 246, 137, 54, 112, 73, 221, 195, 167, 110, 87, 162, 92, 134, 27, 96, 220, 244, 61, 171, 128, 13, 50, 68, 214, 205, 163, 99, 161, 118, 103, 10, 223, 213, 115, 224, 187, 193}

	address, err := builder.GetAddressFromPublicKey(pk)
	require.NoError(t, err)
	require.Equal(t, xc.Address("rZ92eN72ju1usfFhbr3bbu1MuNyRJWPne71pW9A6BZbjLpVr6P9eURoof3qMdkn3FnzYv7EA1kjef5HCVdJ5yhpbF5DwkFDBzDsD11DpBar8w8P4qWTRP9Xw72AFVnugFUt"), address)
}

func TestGetPublicKeyFromAddress(t *testing.T) {
	skBytes, err := hex.DecodeString("1d25811b76f43c86d59d757622773b2969ee71270ea810a42deda024e0cf896a")
	require.NoError(t, err)

	var blsKey bls.PrivateKey[bls.G2]
	err = blsKey.UnmarshalBinary(skBytes)
	require.NoError(t, err)
	pk := blsKey.PublicKey()
	pkBytes, err := pk.MarshalBinary()
	require.NoError(t, err)

	addr := xc.Address("rZ92eN72ju1usfFhbr3bbu1MuNyRJWPne71pW9A6BZbjLpVr6P9eURoof3qMdkn3FnzYv7EA1kjef5HCVdJ5yhpbF5DwkFDBzDsD11DpBar8w8P4qWTRP9Xw72AFVnugFUt")
	addrPk, err := address.GetPublicKeyFromAddress(addr)
	require.NoError(t, err)

	addrPkBytes, err := addrPk.MarshalBinary()
	require.NoError(t, err)

	// Raw `bls.PublicKey` can differ, we have to compare raw bytes
	require.Equal(t, pkBytes, addrPkBytes)
}
