package filecoin

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/filecoin/address"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	// Filecoin addresses must start with "f" (mainnet) or "t" (testnet)
	if !strings.HasPrefix(addrStr, "f") && !strings.HasPrefix(addrStr, "t") {
		return fmt.Errorf("invalid filecoin address %s: must start with f or t prefix", addr)
	}

	// Validate that the second character is a valid protocol (0-4)
	if len(addrStr) < 2 {
		return fmt.Errorf("invalid filecoin address %s: too short", addr)
	}

	_, err := address.AddressToBytes(addrStr)
	if err != nil {
		return fmt.Errorf("invalid filecoin address %s: %w", addr, err)
	}

	return nil
}
