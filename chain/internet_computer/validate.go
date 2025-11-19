package internet_computer

import (
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	// Determine address format
	format, ok := address.GetAddressType(addr)
	if !ok {
		return fmt.Errorf("invalid internet computer address %s: unknown format", addr)
	}

	switch format {
	case address.FormatIcp:
		return validateIcpAddress(addrStr, addr)
	case address.FormatIcrc1:
		return validateIcrc1Address(addrStr, addr)
	default:
		return fmt.Errorf("invalid internet computer address %s: unsupported format %s", addr, format)
	}
}

func validateIcpAddress(addrStr string, addr xc.Address) error {
	// ICP addresses are hex-encoded and should be exactly 64 characters (32 bytes)
	decoded, err := hex.DecodeString(addrStr)
	if err != nil {
		return fmt.Errorf("invalid icp address %s: invalid hex encoding: %w", addr, err)
	}

	if len(decoded) != 32 {
		return fmt.Errorf("invalid icp address %s: must be 32 bytes (64 hex characters), got %d bytes", addr, len(decoded))
	}

	return nil
}

func validateIcrc1Address(addrStr string, addr xc.Address) error {
	// ICRC1 addresses must contain dashes
	if !strings.Contains(addrStr, "-") {
		return fmt.Errorf("invalid icrc1 address %s: must contain dashes", addr)
	}

	// Use the existing Decode function which validates:
	// - Segment lengths
	// - Base32 encoding
	// - CRC32 checksum
	_, err := address.Decode(addrStr)
	if err != nil {
		return fmt.Errorf("invalid icrc1 address %s: %w", addr, err)
	}

	return nil
}
