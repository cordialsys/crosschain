package kaspa

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/kaspanet/kaspad/util"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	require.NotEmpty(chainCfg.ChainPrefix, "chain_prefix is required")
}

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	prefixInt, _ := cfg.ChainPrefix.AsInt()
	prefix := util.Bech32Prefix(prefixInt)

	_, err := util.DecodeAddress(addrStr, prefix)
	if err != nil {
		return fmt.Errorf("invalid kaspa address %s: %w", address, err)
	}

	return nil
}
