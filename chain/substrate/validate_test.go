package substrate_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Substrate addresses (different chain prefixes)
		{
			name:      "Substrate - valid Polkadot address (prefix 0)",
			address:   "1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4F",
			wantError: false,
		},
		{
			name:      "Substrate - valid Kusama address (prefix 2)",
			address:   "CpjsLDC1JFyrhm3ftC9Gs4QoyrkHKhZKtK7YqGTRFtTafgp",
			wantError: false,
		},
		{
			name:      "Substrate - valid Westend address (prefix 42)",
			address:   "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY",
			wantError: false,
		},
		{
			name:      "Substrate - valid generic substrate address (prefix 42)",
			address:   "5DAAnrj7VHTznn2AWBemMuyBwZWs6FNFjdyVXUeYum3PTXFy",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "Substrate - invalid base58",
			address:   "0000000000000000000000000000000000000000000",
			wantError: true,
			errorMsg:  "invalid substrate address",
		},
		{
			name:      "Substrate - too short",
			address:   "1a1LcBX6hGPKg5aQ",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "Substrate - invalid checksum",
			address:   "1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckhea4sz3P1PvL3oc4G",
			wantError: true,
			errorMsg:  "invalid substrate address",
		},
		{
			name:      "Substrate - address with space in middle",
			address:   "1a1LcBX6hGPKg5aQ6DXZpAHCCzWjckh a4sz3P1PvL3oc4F",
			wantError: true,
		},
		{
			name:      "Substrate - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "invalid substrate address",
		},
		{
			name:      "Substrate - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "invalid substrate address",
		},
		{
			name:      "Substrate - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "invalid substrate address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.DOT,
			}

			err := substrate.ValidateAddress(cfg, tt.address)

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
