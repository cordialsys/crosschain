package cardano

import (
	"fmt"
	"slices"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano/address"
	"github.com/cosmos/btcutil/bech32"
)

// Changed max length from 90 to 256 to support longer addresses
func DecodeToBase256(bech string) (string, []byte, error) {
	hrp, data, err := bech32.Decode(bech, 256)
	if err != nil {
		return "", nil, err
	}
	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, err
	}
	return hrp, converted, nil
}

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	addrStr := string(addr)

	// Decode bech32 address using btcd's bech32 library which supports longer addresses
	hrp, decoded, err := DecodeToBase256(addrStr)
	if err != nil {
		return fmt.Errorf("invalid cardano address %s: %w", addr, err)
	}

	// Check network and address type
	isMainnet := cfg.Network == "" || strings.ToLower(cfg.Network) == "mainnet"

	// Validate HRP (Human Readable Part) based on network
	var validHRPs []string
	if isMainnet {
		validHRPs = address.ValidMainnetHRPs
	} else {
		validHRPs = address.ValidTestnetHRPs
	}

	isValidHRP := slices.Contains(validHRPs, hrp)
	if !isValidHRP {
		return fmt.Errorf("invalid cardano address %s: wrong network prefix, expected one of %v, got %s", addr, validHRPs, hrp)
	}

	if len(decoded) < 20 || len(decoded) > 72 {
		return fmt.Errorf("cardano address has invalid length %d", len(decoded))
	}

	return nil
}
