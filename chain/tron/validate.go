package tron

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)
	decoded, _, err := base58.CheckDecode(addrStr)
	if err != nil {
		return fmt.Errorf("invalid tron address %s: invalid base58check encoding: %w", address, err)
	}

	// Decoded address should be 20 bytes (160 bits)
	if len(decoded) != 20 {
		return fmt.Errorf("invalid tron address %s: decoded address must be 20 bytes (got %d)", address, len(decoded))
	}

	return nil
}
