package solana_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Solana addresses
		{
			name:      "Solana - valid address",
			address:   "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
			wantError: false,
		},
		{
			name:      "Solana - valid address (system program)",
			address:   "11111111111111111111111111111111",
			wantError: false,
		},
		{
			name:      "Solana - valid address (token program)",
			address:   "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			wantError: false,
		},
		{
			name:      "Solana - valid address (associated token program)",
			address:   "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL",
			wantError: false,
		},
		{
			name:      "Solana - valid address (USDC mint)",
			address:   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "Solana - address with space in middle",
			address:   "Hzn3n914JaSpnxo5mBbmuC mGL6mxWN9Ac2HzEXFSGtb",
			wantError: true,
		},
		{
			name:      "Solana - invalid base58 (contains 0)",
			address:   "0000000000000000000000000000000000000000000",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "Solana - too short",
			address:   "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN",
			wantError: true,
			errorMsg:  "must be 32 bytes",
		},
		{
			name:      "Solana - too long",
			address:   "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtbExtended",
			wantError: true,
			errorMsg:  "must be 32 bytes",
		},
		{
			name:      "Solana - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "must be 32 bytes",
		},
		{
			name:      "Solana - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "Solana - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.SOL,
			}

			err := solana.ValidateAddress(cfg, tt.address)

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
