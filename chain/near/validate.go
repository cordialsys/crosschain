package near

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/stretchr/testify/require"
)

const (
	MainnetSuffix = "near"
	TestnetSuffix = "testnet"
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	// add chain-specific validation here:
	require.NotEmpty(chainCfg.Chain, ".chain should be set")
}

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	straddress := string(address)
	isImplicitAddress := !strings.Contains(straddress, ".")
	isMainnet := cfg.Network == string(config.Mainnet)

	if isImplicitAddress {
		return IsValidImplicitNearAccountId(straddress)
	} else {
		return IsValidNamedNearAccountId(straddress, isMainnet)
	}
}

func IsValidImplicitNearAccountId(account string) error {
	if len(account) != 64 {
		return fmt.Errorf("implicit address should be of 64 length")
	}

	_, err := hex.DecodeString(account)
	return err
}

func IsValidNamedNearAccountId(account string, isMainnet bool) error {
	if len(account) < 5 || len(account) > 64 { // min 5: a.near
		return fmt.Errorf("invalid NEAR account length, min: 5, max: 64")
	}

	// 2. Split by '.'
	parts := strings.Split(account, ".")
	if len(parts) < 2 {
		// Must have at least one '.' for network suffix
		return fmt.Errorf("invalid number of parts for named account, expected at least 2")
	}

	// 3. Validate suffix
	suffix := parts[len(parts)-1]
	validSuffix := (isMainnet && suffix == MainnetSuffix) || (!isMainnet && suffix == TestnetSuffix)
	if !validSuffix {
		return fmt.Errorf("expected either 'testnet' or 'near' suffix, depending on network")
	}

	// 4. Validate each part except suffix
	for _, part := range parts[:len(parts)-1] {
		matched, _ := regexp.MatchString(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$`, part)
		if !matched {
			return fmt.Errorf("invalid characters included in part")
		}
	}

	return nil
}
