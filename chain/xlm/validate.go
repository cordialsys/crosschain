package xlm

import (
	"encoding/base32"
	"encoding/binary"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/address"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	if len(addrStr) != 56 && len(addrStr) != 69 {
		return fmt.Errorf("invalid xlm address %s: must be 56 characters for G addresses and 69 characters for M addresses (got %d)", addr, len(addrStr))
	}

	// Decode base32 (without padding)
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(addrStr)
	if err != nil {
		return fmt.Errorf("invalid xlm address %s: invalid base32 encoding: %w", addr, err)
	}

	// Verify checksum (last 2 bytes)
	payload := decoded[:len(decoded)-2]
	expectedChecksum := address.Checksum(payload)
	actualChecksum := binary.LittleEndian.Uint16(decoded[len(decoded)-2:])

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("invalid xlm address %s: checksum mismatch", addr)
	}

	return nil
}
