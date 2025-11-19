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

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	// TODO: Implement address validation
	// Common things to validate:
	// 1. Check prefix/format (e.g., "0x" for EVM, "cosmos" for Cosmos, etc.)
	// 2. Validate length
	// 3. Decode and validate encoding (base58, base32, hex, etc.)
	// 4. Validate checksum if applicable
	return nil
}
