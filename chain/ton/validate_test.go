package ton_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton"
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
		// Valid TON addresses - mainnet (user-friendly format)
		{
			name:      "TON - valid mainnet address (EQ prefix)",
			address:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
			network:   "mainnet",
			wantError: false,
		},
		{
			name:      "TON - valid mainnet address (UQ prefix)",
			address:   "UQANCZLrRHVnenvs31J26Y6vUcirln0-6zs_U18w93WaN2da",
			network:   "mainnet",
			wantError: false,
		},
		// Valid TON addresses - testnet (user-friendly format)
		{
			name:      "TON - valid testnet address (kQ prefix)",
			address:   "kQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSq08",
			network:   "testnet",
			wantError: false,
		},
		{
			name:      "TON - valid testnet address (0Q prefix)",
			address:   "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5",
			network:   "testnet",
			wantError: false,
		},
		// Valid TON addresses - raw format
		{
			name:      "TON - valid raw format address",
			address:   "0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A",
			network:   "mainnet",
			wantError: false,
		},
		{
			name:      "TON - valid raw format address (testnet)",
			address:   "0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A",
			network:   "testnet",
			wantError: false,
		},
		{
			name:      "TON - invalid prefix",
			address:   "XQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "TON - invalid base64 encoding",
			address:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2!!!",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "TON - too short",
			address:   "EQAjflEZ",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "TON - invalid checksum",
			address:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha1",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "TON - address with space in middle",
			address:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvH TT6hVwTW95EGVQfeSha2",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "TON - invalid raw format (missing colon)",
			address:   "0237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A",
			network:   "mainnet",
			wantError: true,
		},
		{
			name:      "TON - invalid raw format (invalid hex)",
			address:   "0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DEZZ",
			network:   "mainnet",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:   xc.TON,
				Network: tt.network,
			}

			err := ton.ValidateAddress(cfg, tt.address)

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
