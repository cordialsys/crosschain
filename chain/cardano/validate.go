package cardano

import (
	"fmt"
	"slices"
	"strings"

	xc "github.com/cordialsys/crosschain"
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

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	// Decode bech32 address using btcd's bech32 library which supports longer addresses
	hrp, decoded, err := DecodeToBase256(addrStr)
	if err != nil {
		return fmt.Errorf("invalid cardano address %s: %w", address, err)
	}

	// Check network and address type
	isMainnet := cfg.Network == "" || strings.ToLower(cfg.Network) == "mainnet"

	// Validate HRP (Human Readable Part) based on network
	var validHRPs []string
	if isMainnet {
		validHRPs = []string{"addr", "stake"}
	} else {
		validHRPs = []string{"addr_test", "stake_test"}
	}

	isValidHRP := slices.Contains(validHRPs, hrp)
	if !isValidHRP {
		return fmt.Errorf("invalid cardano address %s: wrong network prefix, expected one of %v, got %s", address, validHRPs, hrp)
	}

	// Validate address length
	// Payment addresses (with stake key): 57 bytes (1 header + 28 payment hash + 28 stake hash)
	// Payment addresses (without stake key): 29 bytes (1 header + 28 payment hash)
	// Stake addresses: 29 bytes (1 header + 28 stake hash)
	validLengths := []int{29, 57}
	isValidLength := slices.Contains(validLengths, len(decoded))
	if !isValidLength {
		return fmt.Errorf("invalid cardano address %s: invalid length %d, expected one of %v", address, len(decoded), validLengths)
	}

	return nil
}
