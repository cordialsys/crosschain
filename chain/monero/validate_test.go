package monero_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/monero"
	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/stretchr/testify/require"
)

const (
	testnetAddr = "9wppZCoZtBD47RB3kY3i5HfYVqMgFXyEx368ts1ugHBHhFEu4U1hUmwhpjTJodDj66RVewXjw6NAzN2px5QbgqsvFFDMhXW"
)

func mainnetEquivalent(t *testing.T) string {
	t.Helper()
	_, spend, view, err := crypto.DecodeAddress(testnetAddr)
	require.NoError(t, err)
	return crypto.GenerateAddressWithPrefix(crypto.MainnetAddressPrefix, spend, view)
}

func TestValidateAddressNetworkMatch(t *testing.T) {
	mainnetAddr := mainnetEquivalent(t)

	cases := []struct {
		name    string
		cfg     *xc.ChainBaseConfig
		addr    string
		wantErr string
	}{
		{"testnet cfg, testnet addr", &xc.ChainBaseConfig{Network: "testnet"}, testnetAddr, ""},
		{"mainnet cfg, mainnet addr", &xc.ChainBaseConfig{Network: "mainnet"}, mainnetAddr, ""},
		{"testnet cfg, mainnet addr", &xc.ChainBaseConfig{Network: "testnet"}, mainnetAddr, "configured for testnet"},
		{"mainnet cfg, testnet addr", &xc.ChainBaseConfig{Network: "mainnet"}, testnetAddr, "configured for mainnet"},
		// chain_id is a secondary testnet signal when Network is unset.
		{"chain_id testnet, mainnet addr", &xc.ChainBaseConfig{ChainID: "testnet"}, mainnetAddr, "configured for testnet"},
		// No network signal: don't block (backwards compatible).
		{"no signal, mainnet addr", &xc.ChainBaseConfig{}, mainnetAddr, ""},
		{"no signal, testnet addr", &xc.ChainBaseConfig{}, testnetAddr, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := monero.ValidateAddress(tc.cfg, xc.Address(tc.addr))
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
