package zcash_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/zcash"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		network   string
		wantError bool
		errorMsg  string
	}{
		// Valid Zcash addresses - mainnet
		{
			name:      "Zcash - valid transparent address (t1)",
			address:   "t1UYsZVJkLPeMjxEtACvSxfWuNmddpWfxzs",
			network:   "mainnet",
			wantError: false,
		},
		{
			name:      "Zcash - valid transparent address 2 (t1)",
			address:   "t1g4xVgMHVsxZWxS6D3SLXNXEAicivXKiAS",
			network:   "mainnet",
			wantError: false,
		},
		{
			name:      "Zcash - valid transparent address 3 (t1)",
			address:   "t1PyjotZbtna7jhzpF4w35wNFX2GGRJFcXM",
			network:   "mainnet",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "Zcash - too short",
			address:   "t1UYsZVJk",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "Zcash - invalid base58 character (0)",
			address:   "t1UYsZVJkLPeMjxEtACvSxfWuNmddpWfxz0",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "Zcash - invalid checksum",
			address:   "t1UYsZVJkLPeMjxEtACvSxfWuNmddpWfxzt",
			network:   "mainnet",
			wantError: true,
			errorMsg:  "checksum",
		},
		{
			name:      "Zcash - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "Zcash - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			network:   "mainnet",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:   xc.ZEC,
				Network: tt.network,
			}

			err := zcash.ValidateAddress(cfg, tt.address)

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
