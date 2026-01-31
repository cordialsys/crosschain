package egld

import (
	"fmt"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/btcutil/bech32"
	"github.com/stretchr/testify/require"
)

const (
	EGLDPrefix        = "erd"
	EGLDAddressLength = 32
)

func Validate(t *testing.T, chainCfg *xc.ChainConfig) {
	require := require.New(t)
	require.NotEmpty(chainCfg.Chain, ".chain should be set")
	require.Equal(xc.EGLD, chainCfg.Chain, "chain should be EGLD")
	require.NotEmpty(chainCfg.IndexerUrl, ".indexer_url (API) should be set for EGLD")
}

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	if addrStr == "" {
		return fmt.Errorf("address cannot be empty")
	}

	if !strings.HasPrefix(addrStr, EGLDPrefix+"1") {
		return fmt.Errorf("invalid EGLD address %s: must start with 'erd1'", address)
	}

	hrp, decoded, err := bech32.DecodeToBase256(addrStr)
	if err != nil {
		return fmt.Errorf("invalid EGLD address %s: failed to decode bech32: %w", address, err)
	}

	if hrp != EGLDPrefix {
		return fmt.Errorf("invalid EGLD address %s: wrong HRP '%s', expected '%s'", address, hrp, EGLDPrefix)
	}

	if len(decoded) != EGLDAddressLength {
		return fmt.Errorf("invalid EGLD address %s: invalid length %d, expected %d", address, len(decoded), EGLDAddressLength)
	}

	return nil
}
