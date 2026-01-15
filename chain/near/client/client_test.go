package client_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/near/client"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c, err := client.NewClient(xc.NewChainConfig(xc.NEAR))
	require.NotNil(t, c)
	require.NoError(t, err)
}
