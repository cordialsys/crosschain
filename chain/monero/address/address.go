package address

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	moneroCrypto "github.com/cordialsys/crosschain/chain/monero/crypto"
)

type AddressBuilder struct {
	cfg     *xc.ChainBaseConfig
	format  xc.AddressFormat
	viewKey []byte // private view key (required for Monero)
	pubView []byte // public view key derived from viewKey
}

func NewAddressBuilder(cfg *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	opts, err := xcaddress.NewAddressOptions(options...)
	if err != nil {
		return nil, err
	}

	viewKeyHex, ok := opts.GetViewKey()
	if !ok || viewKeyHex == "" {
		return nil, fmt.Errorf("monero address builder requires a view key (set chain.view_key or pass via xcaddress.OptionViewKey)")
	}
	viewKey, err := hex.DecodeString(viewKeyHex)
	if err != nil || len(viewKey) != 32 {
		return nil, fmt.Errorf("monero view key must be 64 hex chars (32 bytes): %w", err)
	}
	pubView, err := moneroCrypto.PublicFromPrivate(viewKey)
	if err != nil {
		return nil, fmt.Errorf("invalid monero view key: %w", err)
	}

	var format xc.AddressFormat
	if f, ok := opts.GetFormat(); ok {
		format = f
	}
	return &AddressBuilder{cfg: cfg, format: format, viewKey: viewKey, pubView: pubView}, nil
}

// GetAddressFromPublicKey derives a Monero address from a 32-byte public spend key.
// The public view key comes from the configured view key (set at builder construction).
//
// When format is "subaddress:N" or "subaddress:M/N", it generates a subaddress.
func (ab *AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	var pubSpend []byte
	switch len(publicKeyBytes) {
	case 32:
		pubSpend = publicKeyBytes
	case 64:
		// Accept 64-byte (spend||view) form for compatibility; ignore the view part
		// since this builder owns the view key.
		pubSpend = publicKeyBytes[:32]
	default:
		return "", fmt.Errorf("monero requires 32-byte public spend key, got %d bytes", len(publicKeyBytes))
	}

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
		addr, err := moneroCrypto.GenerateSubaddress(ab.viewKey, pubSpend, index)
		if err != nil {
			return "", fmt.Errorf("failed to generate subaddress: %w", err)
		}
		return xc.Address(addr), nil
	}

	addr := moneroCrypto.GenerateAddressWithPrefix(prefix, pubSpend, ab.pubView)
	return xc.Address(addr), nil
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
