package hyperliquid

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	// add chain-specific validation here:
	require.NotEmpty(chainCfg.Chain, ".chain should be set")
}

// ValidateAddress validates a Hyperliquid address by delegating to the EVM validation
func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	return evm.ValidateAddress(cfg, address)
}
