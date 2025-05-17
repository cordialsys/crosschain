package kaspa

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	require.NotEmpty(chainCfg.ChainPrefix, "chain_prefix is required")
}
