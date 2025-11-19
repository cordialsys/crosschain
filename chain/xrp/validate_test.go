package xrp_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid XRP addresses
		{
			name:      "XRP - valid address 1",
			address:   "rLETt614usCXtkc8YcQmrzachrCaDjACjP",
			wantError: false,
		},
		{
			name:      "XRP - valid address 2",
			address:   "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
			wantError: false,
		},
		{
			name:      "XRP - valid X-address (mainnet)",
			address:   "XV5sbjUmgPpvXv4ixFWZ5ptAYZ6PD28Sq49uo34VyjnmK5H",
			wantError: false,
		},
		{
			name:      "XRP - valid X-address (testnet)",
			address:   "TVd2rqMkYL2AyS97NdELcpeiprNBjwLZzuUG5rZnaewsahi",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "XRP - missing r prefix",
			address:   "LETt614usCXtkc8YcQmrzachrCaDjACjP1",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "XRP - too short",
			address:   "rPT1Sjq2YGr",
			wantError: true,
			errorMsg:  "decoded address must be at least 25 bytes",
		},
		{
			name:      "XRP - invalid base58 character (O)",
			address:   "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYeO",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "XRP - invalid checksum",
			address:   "rLETt614usCXtkc8YcQmrzachrCaDjACjQ",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "XRP - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "XRP - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
		{
			name:      "XRP - stellar address",
			address:   "GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4",
			wantError: true,
			errorMsg:  "invalid base58 encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.XRP,
			}

			err := xrp.ValidateAddress(cfg, tt.address)

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
