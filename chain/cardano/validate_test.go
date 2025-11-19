package cardano_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		network   string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Mainnet - Payment addresses (legacy format - without stake key)
		{
			name:      "ADA mainnet - legacy payment address",
			network:   "mainnet",
			address:   "addr1wy42rt3rdptdaa2lwlntkx49ksuqrmqqjlu7pf5l5f8upmgj3gq2m",
			wantError: false,
		},
		{
			name:      "ADA mainnet - payment address normal",
			network:   "mainnet",
			address:   "addr1gx2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzer5pnz75xxcrzqf96k",
			wantError: false,
		},
		{
			name:      "ADA mainnet - payment address with stake key",
			network:   "mainnet",
			address:   "addr1q9y8tu07chm4g5djr33tye2ckvdnl2cm7s6u7p5mqjs8wfalpcf5mc8ncefe92rj4gz6kwlzxrj54wy25c7c44vvdl9qeyswq3",
			wantError: false,
		},
		// Mainnet - Stake addresses
		{
			name:      "ADA mainnet - stake address",
			network:   "mainnet",
			address:   "stake1u83magsw5xtq0zn2ynu9fqg5ugkhjfkqs0sqgwq3aqlvacsnwl38y",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "ADA mainnet - address with space in middle",
			network:   "mainnet",
			address:   "addr1wy42rt3rdptdaa2lwlntkx4 ksuqrmqqjlu7pf5l5f8upmgj3gq2m",
			wantError: true,
		},
		{
			name:      "ADA mainnet - bitcoin address",
			network:   "mainnet",
			address:   "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
			wantError: true,
			errorMsg:  "invalid cardano address",
		},
		{
			name:      "ADA mainnet - ethereum address",
			network:   "mainnet",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "invalid cardano address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:   xc.ADA,
				Network: tt.network,
			}

			err := cardano.ValidateAddress(cfg, tt.address)

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
