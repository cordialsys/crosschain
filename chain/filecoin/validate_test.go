package filecoin_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/filecoin"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Filecoin addresses - Mainnet
		{
			name:      "Filecoin - valid mainnet secp256k1 address",
			address:   "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			wantError: false,
		},
		{
			name:      "Filecoin - valid mainnet actor address",
			address:   "f2kbv57glniayy75fausbk7cc3xvrsb2bgvcqscwy",
			wantError: false,
		},
		{
			name:      "Filecoin - valid mainnet ID address",
			address:   "f0143103",
			wantError: false,
		},
		// Valid Filecoin addresses - Testnet
		{
			name:      "Filecoin - valid testnet secp256k1 address",
			address:   "t13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			wantError: false,
		},
		{
			name:      "Filecoin - valid testnet secp256k1 address 2",
			address:   "t1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
			wantError: false,
		},
		{
			name:      "Filecoin - valid testnet BLS address",
			address:   "t3vvmn62lofvhjd2ugzca6sof2j2ubwok6cj4xxbfzz4yuxfkgobpihhd2thlanmsh3w2ptld2gqkn2jvlss4a",
			wantError: false,
		},
		{
			name:      "Filecoin - valid testnet ID address",
			address:   "t0143103",
			wantError: false,
		},
		{
			name:      "Filecoin - valid testnet ID address (single digit)",
			address:   "t01",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "Filecoin - missing prefix",
			address:   "13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			wantError: true,
			errorMsg:  "must start with f or t prefix",
		},
		{
			name:      "Filecoin - wrong prefix",
			address:   "x13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			wantError: true,
			errorMsg:  "must start with f or t prefix",
		},
		{
			name:      "Filecoin - too short",
			address:   "f",
			wantError: true,
			errorMsg:  "too short",
		},
		{
			name:      "Filecoin - invalid protocol (5)",
			address:   "f5invalidprotocol",
			wantError: true,
			errorMsg:  "unsupported protocol",
		},
		{
			name:      "Filecoin - invalid protocol (9)",
			address:   "t9invalidprotocol",
			wantError: true,
			errorMsg:  "unsupported protocol",
		},
		{
			name:      "Filecoin - delegated address (not supported)",
			address:   "t410fxgo7645dlyjza2s5ft67pidjhxc5qzeqsspyzjq",
			wantError: true,
			errorMsg:  "unsupported protocol",
		},
		{
			name:      "Filecoin - invalid base32 encoding",
			address:   "f1invalid-base32-chars!@#$%",
			wantError: true,
			errorMsg:  "failed to decode address",
		},
		{
			name:      "Filecoin - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "must start with f or t prefix",
		},
		{
			name:      "Filecoin - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "must start with f or t prefix",
		},
		{
			name:      "Filecoin - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "must start with f or t prefix",
		},
		{
			name:      "Filecoin - EOS address",
			address:   "EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError: true,
			errorMsg:  "must start with f or t prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.FIL,
			}

			err := filecoin.ValidateAddress(cfg, tt.address)

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
