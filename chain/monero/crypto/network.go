package crypto

import (
	"fmt"
	"strings"
)

// NetworkForPrefix returns the Monero network ("mainnet" or "testnet") an
// address prefix belongs to, and whether the prefix is recognized. Stagenet is
// not represented (this implementation does not support it).
func NetworkForPrefix(prefix byte) (network string, known bool) {
	switch prefix {
	case MainnetAddressPrefix, MainnetIntegratedPrefix, MainnetSubaddressPrefix:
		return "mainnet", true
	case TestnetAddressPrefix, TestnetIntegratedPrefix, TestnetSubaddressPrefix:
		return "testnet", true
	default:
		return "", false
	}
}

// CheckAddressNetwork rejects an address whose network (encoded in its prefix
// byte) does not match the chain's configured network. It only flags a definite
// mismatch: an unrecognized prefix, or a config with no clear mainnet/testnet
// signal, returns nil so this never produces a false rejection.
func CheckAddressNetwork(prefix byte, chainNetwork, chainID string) error {
	addrNet, known := NetworkForPrefix(prefix)
	if !known {
		return nil
	}
	var cfgNet string
	switch {
	case strings.EqualFold(chainNetwork, "mainnet"):
		cfgNet = "mainnet"
	case strings.EqualFold(chainNetwork, "testnet"), strings.EqualFold(chainID, "testnet"):
		cfgNet = "testnet"
	}
	if cfgNet == "" {
		return nil
	}
	if cfgNet != addrNet {
		return fmt.Errorf("address is a Monero %s address but the chain is configured for %s", addrNet, cfgNet)
	}
	return nil
}
