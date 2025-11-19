package evm_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		chain     string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid EVM addresses
		{
			name:      "EVM - valid address (lowercase)",
			chain:     string(xc.ETH),
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fce1",
			wantError: false,
		},
		{
			name:      "EVM - valid address (uppercase)",
			chain:     string(xc.ETH),
			address:   "0x5891906FEF64A5AE924C7FC5ED48C0F64A55FCE1",
			wantError: false,
		},
		{
			name:      "EVM - valid address (mixed case)",
			chain:     string(xc.ETH),
			address:   "0x5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1",
			wantError: false,
		},
		{
			name:      "EVM - valid address (all zeros)",
			chain:     string(xc.ETH),
			address:   "0x0000000000000000000000000000000000000000",
			wantError: false,
		},
		{
			name:      "EVM - valid address (all F's)",
			chain:     string(xc.ETH),
			address:   "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
			wantError: false,
		},
		{
			name:      "EVM - valid XDC address (lowercase)",
			chain:     string(xc.XDC),
			address:   "xdc5891906fef64a5ae924c7fc5ed48c0f64a55fce1",
			wantError: false,
		},
		{
			name:      "EVM - valid XDC address (mixed case)",
			chain:     string(xc.XDC),
			address:   "xdc5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "EVM - missing prefix",
			chain:     string(xc.ETH),
			address:   "5891906fef64a5ae924c7fc5ed48c0f64a55fce1",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "EVM - space in address",
			chain:     string(xc.ETH),
			address:   "0x5891906fef64a5ae92 c7fc5ed48c0f64a55fce1",
			wantError: true,
			errorMsg:  "invalid hex encoding",
		},
		{
			name:      "EVM - wrong prefix",
			chain:     string(xc.ETH),
			address:   "0y5891906fef64a5ae924c7fc5ed48c0f64a55fce1",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "EVM - too short (39 chars)",
			chain:     string(xc.ETH),
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fce",
			wantError: true,
			errorMsg:  "must be 40 hex characters",
		},
		{
			name:      "EVM - too long (41 chars)",
			chain:     string(xc.ETH),
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fce11",
			wantError: true,
			errorMsg:  "must be 40 hex characters",
		},
		{
			name:      "EVM - invalid hex characters",
			chain:     string(xc.ETH),
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fceg",
			wantError: true,
			errorMsg:  "invalid hex encoding",
		},
		{
			name:      "EVM - invalid hex characters (z)",
			chain:     string(xc.ETH),
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fcez",
			wantError: true,
			errorMsg:  "invalid hex encoding",
		},
		{
			name:      "EVM - bitcoin address",
			chain:     string(xc.ETH),
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "EVM - cosmos address",
			chain:     string(xc.ETH),
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "EVM - EOS address",
			chain:     string(xc.ETH),
			address:   "EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
		{
			name:      "EVM - cardano address",
			chain:     string(xc.ETH),
			address:   "addr1vxjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s02dyt3",
			wantError: true,
			errorMsg:  "must start with 0x prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.NativeAsset(tt.chain),
			}

			err := evm.ValidateAddress(cfg, tt.address)

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
