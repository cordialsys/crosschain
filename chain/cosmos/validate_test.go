package cosmos_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		chain     xc.NativeAsset
		prefix    string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Cosmos Hub (ATOM)
		{
			name:      "ATOM - valid address",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: false,
		},
		// Terra (LUNA)
		{
			name:      "LUNA - valid address",
			chain:     xc.LUNA,
			prefix:    "terra",
			address:   "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			wantError: false,
		},
		{
			name:      "LUNA - another valid address",
			chain:     xc.LUNA,
			prefix:    "terra",
			address:   "terra1evfrnqr9l5yxjp7ejektl2xmjdqlz08tuundzw",
			wantError: false,
		},
		// Injective (INJ)
		{
			name:      "INJ - valid address",
			chain:     xc.INJ,
			prefix:    "inj",
			address:   "inj12szvunq39ky0lq20t9cy3tcs49n8k56v9q38fj",
			wantError: false,
		},
		// XPLA
		{
			name:      "XPLA - valid address",
			chain:     xc.XPLA,
			prefix:    "xpla",
			address:   "xpla1r56x9533ntqtlsd99cth48fhyjf82gfstgvk9m",
			wantError: false,
		},
		{
			name:      "XPLA - another valid address",
			chain:     xc.XPLA,
			prefix:    "xpla",
			address:   "xpla18tqp4j6ndm9fmly4t5mzgwh5zeg9rqrpm7zfnp",
			wantError: false,
		},
		// SEI
		{
			name:      "SEI - valid address",
			chain:     xc.SEI,
			prefix:    "sei",
			address:   "sei1auf4yetx9z3lq5f93d8p8mm4j3lpt9s077m455",
			wantError: false,
		},
		// Celestia (TIA)
		{
			name:      "TIA - valid address",
			chain:     xc.TIA,
			prefix:    "celestia",
			address:   "celestia1cl5k4awzka64ck974j4kshzezhmznpg66724xj",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "ATOM - invalid bech32",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "cosmos1invalid",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
		{
			name:      "ATOM - wrong prefix (terra address)",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
		{
			name:      "LUNA - wrong prefix (cosmos address)",
			chain:     xc.LUNA,
			prefix:    "terra",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
		{
			name:      "ATOM - invalid checksum",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vz1",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
		{
			name:      "ATOM - too short",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "cosmos1",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
		{
			name:      "ATOM - bitcoin address",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
		{
			name:      "ATOM - ethereum address",
			chain:     xc.ATOM,
			prefix:    "cosmos",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "invalid cosmos address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:       tt.chain,
				ChainPrefix: xc.StringOrInt(tt.prefix),
			}

			err := cosmos.ValidateAddress(cfg, tt.address)

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
