package cosmos

import (
	"fmt"
	"slices"
	"testing"

	xc "github.com/cordialsys/crosschain"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	require.NotEmpty(chainCfg.ChainPrefix, "chain_prefix is required")
}

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	// Get expected prefix from config
	expectedPrefix := string(cfg.ChainPrefix)
	if expectedPrefix == "" {
		return fmt.Errorf("chain prefix not configured for %s", cfg.Chain)
	}

	// Decode bech32 address
	bz, err := sdk.GetFromBech32(addrStr, expectedPrefix)
	if err != nil {
		return fmt.Errorf("invalid cosmos address %s: %w", address, err)
	}

	// Validate address length (should be 20 bytes for most Cosmos chains)
	// Some chains may use different lengths, but 20 is standard
	validLengths := []int{20, 32}
	isValidLength := slices.Contains(validLengths, len(bz))
	if !isValidLength {
		return fmt.Errorf("invalid cosmos address %s: invalid length %d, expected one of %v", address, len(bz), validLengths)
	}

	return nil
}
