package evm

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	if chainCfg.ChainID != "" {
		_, ok := chainCfg.ChainID.AsInt()
		require.True(t, ok, fmt.Sprintf("%s should have a valid integer chain_id (%s)", chainCfg.Chain, chainCfg.ChainID))
	}
}

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	// EVM addresses use "0x" prefix (standard) or "xdc" prefix (XinFin/XDC network only)
	var hexPart string
	if strings.HasPrefix(addrStr, "0x") {
		hexPart = strings.TrimPrefix(addrStr, "0x")
	} else if strings.HasPrefix(addrStr, "xdc") && cfg.Chain == xc.XDC {
		hexPart = strings.TrimPrefix(addrStr, "xdc")
	} else {
		return fmt.Errorf("invalid evm address %s: must start with 0x prefix", address)
	}

	// EVM addresses should be exactly 40 hex characters (20 bytes)
	if len(hexPart) != common.AddressLength*2 {
		return fmt.Errorf("invalid evm address %s: must be 40 hex characters (got %d)", address, len(hexPart))
	}

	// Validate hex encoding (case-insensitive, no checksum validation)
	_, err := hex.DecodeString(hexPart)
	if err != nil {
		return fmt.Errorf("invalid evm address %s: invalid hex encoding: %w", address, err)
	}

	return nil
}
