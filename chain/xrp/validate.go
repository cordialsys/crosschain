package xrp

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/address"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	// Decode base58 using XRP's custom alphabet
	decoded, err := address.DecodeBase58(addrStr)
	if err != nil {
		return fmt.Errorf("invalid xrp address %s: invalid base58 encoding: %w", addr, err)
	}

	// Decoded should be at least 25 bytes: 1 type prefix + 20 account ID + 4 checksum
	// X-addresses are longer (33 bytes): 1 type prefix + 20 account ID + 8 destination tag + 4 checksum
	if len(decoded) < 25 {
		return fmt.Errorf("invalid xrp address %s: decoded address must be at least 25 bytes (got %d)", addr, len(decoded))
	}

	// Verify checksum (last 4 bytes)
	payload := decoded[:len(decoded)-4]
	expectedChecksum := decoded[len(decoded)-4:]

	// Calculate checksum: first 4 bytes of SHA256(SHA256(payload))
	firstSHA256 := sha256.Sum256(payload)
	secondSHA256 := sha256.Sum256(firstSHA256[:])
	actualChecksum := secondSHA256[:4]

	if !bytes.Equal(actualChecksum, expectedChecksum) {
		return fmt.Errorf("invalid xrp address %s: checksum mismatch", addr)
	}

	return nil
}
