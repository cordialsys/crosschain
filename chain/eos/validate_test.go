package eos_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/eos"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid EOS public key addresses
		{
			name:      "EOS - valid EOS public key",
			address:   "EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError: false,
		},
		// Note: PUB_K1 format is supported but we don't have a verified valid example to test
		// Valid EOS account names
		{
			name:      "EOS - valid account name (eosio)",
			address:   "eosio",
			wantError: false,
		},
		{
			name:      "EOS - valid account name (alice)",
			address:   "alice",
			wantError: false,
		},
		{
			name:      "EOS - valid account name (bob12345)",
			address:   "bob12345",
			wantError: false,
		},
		{
			name:      "EOS - valid account name with period",
			address:   "alice.token",
			wantError: false,
		},
		{
			name:      "EOS - valid account name (12 chars)",
			address:   "abcdefghijkl",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "EOS - missing prefix (treated as invalid account name - too long)",
			address:   "6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		{
			name:      "EOS - wrong prefix (treated as invalid account name)",
			address:   "BTC6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		{
			name:      "EOS - malformed base58",
			address:   "EOSnot-valid",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "EOS - too short",
			address:   "EOS6bji",
			wantError: true,
			errorMsg:  "invalid length",
		},
		{
			name:      "EOS - invalid checksum",
			address:   "EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1t",
			wantError: true,
			errorMsg:  "invalid checksum",
		},
		{
			name:      "EOS - bitcoin address (too long for account name)",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		{
			name:      "EOS - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		{
			name:      "EOS - ethereum address (too long for account name)",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		{
			name:      "EOS - cardano address (too long for account name)",
			address:   "addr1vxjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s02dyt3",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		// Invalid EOS account names
		{
			name:      "EOS - account name too long (13 chars)",
			address:   "abcdefghijklm",
			wantError: true,
			errorMsg:  "must be between 1 and 12 characters",
		},
		{
			name:      "EOS - account name with uppercase",
			address:   "Alice",
			wantError: true,
			errorMsg:  "only lowercase a-z, numbers 1-5, and period are allowed",
		},
		{
			name:      "EOS - account name with invalid number (0)",
			address:   "alice0",
			wantError: true,
			errorMsg:  "only lowercase a-z, numbers 1-5, and period are allowed",
		},
		{
			name:      "EOS - account name with invalid number (6)",
			address:   "alice6",
			wantError: true,
			errorMsg:  "only lowercase a-z, numbers 1-5, and period are allowed",
		},
		{
			name:      "EOS - account name starting with number",
			address:   "1alice",
			wantError: true,
			errorMsg:  "cannot start with number or period",
		},
		{
			name:      "EOS - account name starting with period",
			address:   ".alice",
			wantError: true,
			errorMsg:  "cannot start with number or period",
		},
		{
			name:      "EOS - account name ending with period",
			address:   "alice.",
			wantError: true,
			errorMsg:  "cannot end with period",
		},
		{
			name:      "EOS - account name with special char",
			address:   "alice-bob",
			wantError: true,
			errorMsg:  "only lowercase a-z, numbers 1-5, and period are allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.EOS,
			}

			err := eos.ValidateAddress(cfg, tt.address)

			if tt.wantError {
				require.Error(err)
				if tt.errorMsg != "" {
					require.Contains(err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(err)
			}
		})
	}
}
