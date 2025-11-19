package eos

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	eosaddress "github.com/cordialsys/crosschain/chain/eos/address"
	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/stretchr/testify/require"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	for _, na := range chainCfg.NativeAssets {
		// all contract IDs should be valid eos contract IDs
		// in format "<contract>/<symbol>"
		_, _, err := tx_input.ParseContractId(&xc.ChainBaseConfig{}, na.ContractId, nil)
		require.NoError(err, "invalid contract ID: %s", na.ContractId)
	}
}

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	// EOS supports three address formats:
	// 1. EOS public keys: "EOS..." (base58 encoded)
	// 2. PUB_K1 public keys: "PUB_K1_..." (base58 encoded)
	// 3. EOS account names: 12 chars or less, lowercase a-z, numbers 1-5, period

	// Check if it's a PUB_K1 public key format
	if strings.HasPrefix(addrStr, "PUB_K1_") {
		return validateEOSPublicKey(addrStr, "PUB_K1_", address)
	}

	// Check if it's an EOS public key format
	if strings.HasPrefix(addrStr, "EOS") {
		return validateEOSPublicKey(addrStr, "EOS", address)
	}

	// Otherwise, validate as EOS account name
	return validateEOSAccountName(addrStr, address)
}

func validateEOSPublicKey(addrStr, prefix string, address xc.Address) error {
	// Remove prefix
	addressWithoutPrefix := addrStr[len(prefix):]

	// Decode base58
	decoded := base58.Decode(addressWithoutPrefix)
	if len(decoded) == 0 {
		return fmt.Errorf("invalid eos address %s: invalid base58 encoding", address)
	}

	// EOS public keys should be 33 bytes (compressed pubkey) + 4 bytes (checksum) = 37 bytes
	if len(decoded) != 37 {
		return fmt.Errorf("invalid eos address %s: invalid length %d, expected 37", address, len(decoded))
	}

	// Extract public key and checksum
	pubkey := decoded[:33]
	providedChecksum := decoded[33:]

	// Verify checksum
	expectedChecksum := eosaddress.Ripemd160Checksum(pubkey, ecc.CurveK1)
	if len(expectedChecksum) < 4 {
		return fmt.Errorf("invalid eos address %s: checksum computation failed", address)
	}

	if !bytes.Equal(providedChecksum, expectedChecksum[:4]) {
		return fmt.Errorf("invalid eos address %s: invalid checksum", address)
	}

	return nil
}

func validateEOSAccountName(name string, address xc.Address) error {
	// EOS account names must be:
	// - 12 characters or less
	// - Only lowercase letters a-z, numbers 1-5, and period (.)
	// - Cannot start with a number or period
	// - Cannot end with a period

	if len(name) == 0 || len(name) > 12 {
		return fmt.Errorf("invalid eos account name %s: must be between 1 and 12 characters", address)
	}

	// Check first character (cannot start with number or period)
	firstChar := name[0]
	if (firstChar >= '1' && firstChar <= '5') || firstChar == '.' {
		return fmt.Errorf("invalid eos account name %s: cannot start with number or period", address)
	}

	// Check last character (cannot end with period)
	if name[len(name)-1] == '.' {
		return fmt.Errorf("invalid eos account name %s: cannot end with period", address)
	}

	// Check all characters are valid (a-z, 1-5, .)
	for _, char := range name {
		isLowercase := char >= 'a' && char <= 'z'
		isNumber := char >= '1' && char <= '5'
		isPeriod := char == '.'

		if !isLowercase && !isNumber && !isPeriod {
			return fmt.Errorf("invalid eos account name %s: only lowercase a-z, numbers 1-5, and period are allowed", address)
		}
	}

	return nil
}
