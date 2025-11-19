package sui

import (
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	if !strings.HasPrefix(addrStr, "0x") {
		return fmt.Errorf("invalid sui address %s: must start with 0x prefix", address)
	}
	hexPart := strings.TrimPrefix(addrStr, "0x")

	if len(hexPart) != ADDRESS_LENGTH {
		return fmt.Errorf("invalid sui address %s: must be %d hex characters (got %d)", address, ADDRESS_LENGTH, len(hexPart))
	}

	_, err := hex.DecodeString(hexPart)
	if err != nil {
		return fmt.Errorf("invalid sui address %s: invalid hex encoding: %w", address, err)
	}

	return nil
}
