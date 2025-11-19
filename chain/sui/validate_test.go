package sui_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Sui addresses
		{
			name:      "Sui - valid address",
			address:   "0x086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8e",
			wantError: false,
		},
		{
			name:      "Sui - valid address (uppercase)",
			address:   "0x086D8E59C3EF72CCC8CBF74C55E7F611B0EE9EBA788C7153924C4E4A32449A8E",
			wantError: false,
		},
		{
			name:      "Sui - valid address (mixed case)",
			address:   "0x086d8E59c3Ef72ccC8cbf74c55e7F611b0ee9eba788c7153924C4e4a32449a8e",
			wantError: false,
		},
		{
			name:      "Sui - valid address (all zeros)",
			address:   "0x0000000000000000000000000000000000000000000000000000000000000000",
			wantError: false,
		},
		{
			name:      "Sui - valid address (all f's)",
			address:   "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "Sui - missing prefix",
			address:   "086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8e",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "Sui - wrong prefix",
			address:   "0y086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8e",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "Sui - too short (63 chars)",
			address:   "0x086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8",
			wantError: true,
			errorMsg:  "must be 64 hex characters",
		},
		{
			name:      "Sui - too long (65 chars)",
			address:   "0x086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449a8e1",
			wantError: true,
			errorMsg:  "must be 64 hex characters",
		},
		{
			name:      "Sui - invalid hex characters",
			address:   "0x086d8e59c3ef72ccc8cbf74c55e7f611b0ee9eba788c7153924c4e4a32449azz",
			wantError: true,
			errorMsg:  "invalid hex encoding",
		},
		{
			name:      "Sui - address with space in middle",
			address:   "0x086d8e59c3ef72ccc8cbf74c55e7f611b0 e9eba788c7153924c4e4a32449a8e",
			wantError: true,
		},
		{
			name:      "Sui - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "Sui - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.SUI,
			}

			err := sui.ValidateAddress(cfg, tt.address)

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
