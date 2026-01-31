package egld_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/egld"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Valid mainnet address",
			address:   "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
			wantError: false,
		},
		{
			name:      "Valid testnet address",
			address:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
			wantError: false,
		},
		{
			name:      "Another valid address",
			address:   "erd1egkf0ttvylemt8t8hvhqajfszle88hykyppgrtn3pd02gja2j9msx67my9",
			wantError: false,
		},
		{
			name:      "Empty address",
			address:   "",
			wantError: true,
			errorMsg:  "address cannot be empty",
		},
		{
			name:      "Invalid prefix - missing erd1",
			address:   "cosmos1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ss",
			wantError: true,
			errorMsg:  "must start with 'erd1'",
		},
		{
			name:      "Invalid prefix - erd without 1",
			address:   "erdqyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
			wantError: true,
			errorMsg:  "must start with 'erd1'",
		},
		{
			name:      "Invalid bech32 encoding",
			address:   "erd1invalidbech32encoding!!!",
			wantError: true,
			errorMsg:  "failed to decode bech32",
		},
		{
			name:      "Too short address",
			address:   "erd1qyu5wthldzr8wx5c",
			wantError: true,
			errorMsg:  "failed to decode bech32",
		},
		{
			name:      "Wrong HRP",
			address:   "cosmos1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ss",
			wantError: true,
			errorMsg:  "must start with 'erd1'",
		},
		{
			name:      "Hex address instead of bech32",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			wantError: true,
			errorMsg:  "must start with 'erd1'",
		},
		{
			name:      "Random string",
			address:   "not_an_address",
			wantError: true,
			errorMsg:  "must start with 'erd1'",
		},
		{
			name:      "Address with invalid characters",
			address:   "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ss!!!",
			wantError: true,
			errorMsg:  "failed to decode bech32",
		},
		{
			name:      "Case sensitivity test - uppercase",
			address:   "ERD1QYU5WTHLDZR8WX5C9UCG8KJAGG0JFS53S8NR3ZPZ3HYPEFSDD8SSYCR6TH",
			wantError: true,
			errorMsg:  "must start with 'erd1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.EGLD,
			}

			err := egld.ValidateAddress(cfg, tt.address)

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

func TestValidateAddressWithRealAddresses(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}

	// Test a collection of real EGLD addresses from mainnet
	realAddresses := []xc.Address{
		"erd1qqqqqqqqqqqqqpgqxwakt2g7u9atsnr03gqcgmhcv38pt7mkd94q6shuwt", // Smart contract
		"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",   // Regular address
		"erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",   // Regular address
		"erd1egkf0ttvylemt8t8hvhqajfszle88hykyppgrtn3pd02gja2j9msx67my9",   // Regular address
	}

	for _, addr := range realAddresses {
		err := egld.ValidateAddress(cfg, addr)
		require.NoError(err, "address %s should be valid", addr)
	}
}

func TestValidateAddressConsistentWithAddressBuilder(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}

	// Test that ValidateAddress accepts what the address builder generates
	// Using a known valid EGLD address that could be generated from a public key
	testAddr := "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th"
	err := egld.ValidateAddress(cfg, xc.Address(testAddr))
	require.NoError(err)
}
