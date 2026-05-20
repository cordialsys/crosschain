package address

import (
	"fmt"
	"strconv"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	moneroCrypto "github.com/cordialsys/crosschain/chain/monero/crypto"
)

type AddressBuilder struct {
	cfg    *xc.ChainBaseConfig
	format xc.AddressFormat
}

func NewAddressBuilder(cfg *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	opts, err := xcaddress.NewAddressOptions(options...)
	if err != nil {
		return nil, err
	}
	var format xc.AddressFormat
	if f, ok := opts.GetFormat(); ok {
		format = f
	}
	return &AddressBuilder{cfg: cfg, format: format}, nil
}

// GetAddressFromPublicKey derives a Monero address from a 64-byte public key
// (publicSpendKey || publicViewKey).
//
// When format is "subaddress:N" or "subaddress:M/N", it generates a subaddress.
// Subaddress generation requires the private view key, which is loaded from
// the XC_PRIVATE_KEY environment variable.
func (ab *AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) != 64 {
		return "", fmt.Errorf("monero requires 64-byte public key (spend||view), got %d bytes", len(publicKeyBytes))
	}

	pubSpend := publicKeyBytes[:32]
	pubView := publicKeyBytes[32:]

	// Determine address prefix based on network
	prefix := moneroCrypto.MainnetAddressPrefix
	if ab.cfg != nil && (string(ab.cfg.ChainID) == "testnet" || ab.cfg.Network == "testnet") {
		prefix = moneroCrypto.TestnetAddressPrefix
	}

	// Check if subaddress format is requested
	formatStr := string(ab.format)
	if strings.HasPrefix(formatStr, "subaddress:") {
		indexStr := strings.TrimPrefix(formatStr, "subaddress:")
		index, err := ParseSubaddressIndex(indexStr)
		if err != nil {
			return "", fmt.Errorf("invalid subaddress format: %w", err)
		}

		// For subaddress derivation we need the private view key.
		// Derive it from the private spend key in the environment.
		privView, err := loadPrivateViewKey()
		if err != nil {
			return "", fmt.Errorf("subaddress generation requires private key: %w", err)
		}

		addr, err := moneroCrypto.GenerateSubaddress(privView, pubSpend, index)
		if err != nil {
			return "", fmt.Errorf("failed to generate subaddress: %w", err)
		}
		return xc.Address(addr), nil
	}

	addr := moneroCrypto.GenerateAddressWithPrefix(prefix, pubSpend, pubView)
	return xc.Address(addr), nil
}

// loadPrivateViewKey returns the fixed shared view key.
// The view key is independent of the spend key in this implementation - it's a
// single shared constant used across all addresses, allowing one LWS registration
// (with the master spend key + view key) to detect deposits to any subaddress.
func loadPrivateViewKey() ([]byte, error) {
	return moneroCrypto.FixedPrivateViewKey, nil
}

// ParseSubaddressIndex parses a format string like "0", "5", "0/3" into a SubaddressIndex.
func ParseSubaddressIndex(format string) (moneroCrypto.SubaddressIndex, error) {
	parts := strings.Split(format, "/")
	switch len(parts) {
	case 1:
		minor, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return moneroCrypto.SubaddressIndex{}, fmt.Errorf("invalid subaddress index: %w", err)
		}
		return moneroCrypto.SubaddressIndex{Major: 0, Minor: uint32(minor)}, nil
	case 2:
		major, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return moneroCrypto.SubaddressIndex{}, fmt.Errorf("invalid major index: %w", err)
		}
		minor, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return moneroCrypto.SubaddressIndex{}, fmt.Errorf("invalid minor index: %w", err)
		}
		return moneroCrypto.SubaddressIndex{Major: uint32(major), Minor: uint32(minor)}, nil
	default:
		return moneroCrypto.SubaddressIndex{}, fmt.Errorf("invalid format: %s (use N or M/N)", format)
	}
}
