package evm

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	if chainCfg.ChainID != "" {
		_, ok := chainCfg.ChainID.AsInt()
		require.True(t, ok, fmt.Sprintf("%s should have a valid integer chain_id (%s)", chainCfg.Chain, chainCfg.ChainID))
	}
}
