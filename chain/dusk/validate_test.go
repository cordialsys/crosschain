package dusk_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/dusk"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Dusk addresses
		{
			name:      "DUSK - valid address",
			address:   "23BAjR6RE263ejcZxvTkASJht6ACpkhVjAyQ55iez7PZJfvTRvWppUfR47PKGdQ5AccdXGM43c4mRrUpcBcQhnpCgbnTc6GVbS93JE2eoinUYz6GqEDVjce99qonGt3pkUza",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "DUSK - malformed base58",
			address:   "not-a-valid-address",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "DUSK - too short",
			address:   "rZ92eN72ju1",
			wantError: true,
			errorMsg:  "invalid length",
		},
		{
			name:      "DUSK - invalid characters",
			address:   "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "DUSK - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "invalid length",
		},
		{
			name:      "DUSK - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "DUSK - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "DUSK - cardano address",
			address:   "addr1vxjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s02dyt3",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.DUSK,
			}

			err := dusk.ValidateAddress(cfg, tt.address)

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
