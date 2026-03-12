package canton

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

// ValidateAddress validates a Canton party ID address format.
// Canton party IDs must be in the format "name::fingerprint" where:
// - name: 1-185 characters, matching [a-zA-Z0-9:-_ ], no consecutive colons
// - fingerprint: starts with "12" followed by 64 hex characters (SHA256 hash)
func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	if addrStr == "" {
		return fmt.Errorf("empty address")
	}

	// Find the :: separator
	parts := strings.Split(addrStr, "::")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Canton party ID format: must contain exactly one '::' separator")
	}

	name := parts[0]
	fingerprint := parts[1]

	// Validate party name
	if err := validatePartyName(name); err != nil {
		return fmt.Errorf("invalid party name: %w", err)
	}

	// Validate fingerprint
	if err := validateFingerprint(fingerprint); err != nil {
		return fmt.Errorf("invalid fingerprint: %w", err)
	}

	return nil
}

// validatePartyName validates the party name component
func validatePartyName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("party name cannot be empty")
	}

	if len(name) > 185 {
		return fmt.Errorf("party name too long: %d characters (maximum 185)", len(name))
	}

	// Check for valid characters: [a-zA-Z0-9:-_ ]
	validChars := regexp.MustCompile(`^[a-zA-Z0-9:\-_ ]+$`)
	if !validChars.MatchString(name) {
		return fmt.Errorf("party name contains invalid characters (allowed: a-zA-Z0-9:-_ )")
	}

	// Check for consecutive colons
	if strings.Contains(name, "::") {
		return fmt.Errorf("party name cannot contain consecutive colons")
	}

	return nil
}

// validateFingerprint validates the fingerprint component
func validateFingerprint(fingerprint string) error {
	// Fingerprint must start with "1220" (SHA-256 multihash prefix) and be followed by 64 hex characters
	if !strings.HasPrefix(fingerprint, "1220") {
		return fmt.Errorf("fingerprint must start with '1220' (SHA-256 multihash prefix)")
	}

	// Remove the "1220" prefix
	hexPart := fingerprint[4:]

	// SHA256 produces 32 bytes = 64 hex characters
	if len(hexPart) != 64 {
		return fmt.Errorf("fingerprint must be 68 characters total (1220 + 64 hex chars), got %d", len(fingerprint))
	}

	// Validate hex encoding
	if _, err := hex.DecodeString(hexPart); err != nil {
		return fmt.Errorf("fingerprint contains invalid hex characters: %w", err)
	}

	return nil
}
