package hedera

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	// add chain-specific validation here:
	fmt.Println("chain id: ", chainCfg.ChainID)
	require.NotEmpty(chainCfg.ChainID, ".ChainId should be set to node id")
}
