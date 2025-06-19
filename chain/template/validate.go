package newchain

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	// add chain-specific validation here:
	require.NotEmpty(chainCfg.Chain, ".chain should be set")
}
