package client_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/client"
	testtypes "github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func TestFetchUnsimulatedInputEmptySmartAccountCall(t *testing.T) {
	server, close := testtypes.MockJSONRPC(t, []string{
		`"0x6"`,
		`"0x123"`,
		`"0x9"`,
		`"0x"`,
		`"0xef01000000000000000000000000000000000000000001"`,
	})
	defer close()

	asset := xc.NewChainConfig(xc.ETH, xc.DriverEVM).WithUrl(server.URL)
	asset.NoGasFees = true
	evmClient, err := client.NewClient(asset)
	require.NoError(t, err)

	from := xc.Address("0x5c2c12cea90f0fb52c1581300fa934a13e1c2421")
	feePayer := xc.Address("0x58a83507bd717fba17715b1326a72b7dbe59a147")
	input, err := evmClient.FetchUnsimulatedInput(t.Context(), from, feePayer, nil)

	require.NoError(t, err)
	require.Equal(t, uint64(0), input.BasicSmartAccountNonce)
	require.Equal(t, uint64(9), input.FeePayerNonce)
	require.Equal(t, feePayer, input.FeePayerAddress)
	require.Equal(t, 5, server.Counter)
}
