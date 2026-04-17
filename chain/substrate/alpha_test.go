package substrate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAlphaContract(t *testing.T) {
	require := require.New(t)

	netuid, err := ParseAlphaContract("18")
	require.NoError(err)
	require.Equal(uint16(18), netuid)

	netuid, err = ParseAlphaContract("0")
	require.NoError(err)
	require.Equal(uint16(0), netuid)

	netuid, err = ParseAlphaContract("65535")
	require.NoError(err)
	require.Equal(uint16(65535), netuid)

	// with whitespace
	netuid, err = ParseAlphaContract(" 64 ")
	require.NoError(err)
	require.Equal(uint16(64), netuid)

	// empty
	_, err = ParseAlphaContract("")
	require.Error(err)

	// not a number
	_, err = ParseAlphaContract("abc")
	require.Error(err)
	require.Contains(err.Error(), "expected a subnet netuid")

	// overflow
	_, err = ParseAlphaContract("70000")
	require.Error(err)

	// negative
	_, err = ParseAlphaContract("-1")
	require.Error(err)

	// old hotkey/netuid format should fail
	_, err = ParseAlphaContract("5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty/18")
	require.Error(err)
}
