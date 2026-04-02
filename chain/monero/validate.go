package monero

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addr := string(address)

	// Monero mainnet addresses are 95 characters (standard) or 106 characters (integrated)
	if len(addr) != 95 && len(addr) != 106 {
		return fmt.Errorf("invalid monero address length: got %d, expected 95 or 106", len(addr))
	}

	prefix, _, _, err := crypto.DecodeAddress(addr)
	if err != nil {
		return fmt.Errorf("invalid monero address: %w", err)
	}

	// Check valid prefix
	switch prefix {
	case crypto.MainnetAddressPrefix, crypto.MainnetIntegratedPrefix, crypto.MainnetSubaddressPrefix,
		crypto.TestnetAddressPrefix, crypto.TestnetIntegratedPrefix, crypto.TestnetSubaddressPrefix:
		// valid
	default:
		return fmt.Errorf("invalid monero address prefix: %d", prefix)
	}

	return nil
}
