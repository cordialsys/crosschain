package bitcoin_cash_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		chain     string
		network   string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Bitcoin Cash mainnet - CashAddr format (with prefix)
		{
			name:      "BCH mainnet - CashAddr P2PKH with prefix",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "bitcoincash:qpm2qsznhks23z7629mms6s4cwef74vcwvy22gdx6a",
			wantError: false,
		},
		{
			name:      "BCH mainnet - CashAddr P2PKH without prefix",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "qpm2qsznhks23z7629mms6s4cwef74vcwvy22gdx6a",
			wantError: false,
		},
		{
			name:      "BCH mainnet - CashAddr P2SH with prefix",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "bitcoincash:ppm2qsznhks23z7629mms6s4cwef74vcwvn0h829pq",
			wantError: false,
		},
		{
			name:      "BCH mainnet - CashAddr P2SH without prefix",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "ppm2qsznhks23z7629mms6s4cwef74vcwvn0h829pq",
			wantError: false,
		},
		// Bitcoin Cash mainnet - Legacy format
		{
			name:      "BCH mainnet - Legacy P2PKH",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "1BpEi6DfDAUFd7GtittLSdBeYJvcoaVggu",
			wantError: false,
		},
		{
			name:      "BCH mainnet - Legacy P2SH",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "3CWFddi6m4ndiGyKqzYvsFYagqDLPVMTzC",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "BCH mainnet - empty address",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "",
			wantError: true,
			errorMsg:  "invalid bitcoin cash address",
		},
		{
			name:      "BCH mainnet - invalid checksum",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "qpm2qsznhks23z7629mms6s4cwef74vcwvy22gdx6b",
			wantError: true,
			errorMsg:  "invalid bitcoin cash address",
		},
		{
			name:      "BCH mainnet - malformed address",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "not-a-valid-address",
			wantError: true,
			errorMsg:  "invalid bitcoin cash address",
		},
		// Note: Bitcoin segwit addresses may be accepted by the decoder as they have similar structure
		// This is a limitation of the validation approach
		{
			name:      "BCH mainnet - legacy invalid checksum",
			chain:     string(xc.BCH),
			network:   "mainnet",
			address:   "1BpEi6DfDAUFd7GtittLSdBeYJvcoaVggu1",
			wantError: true,
			errorMsg:  "invalid bitcoin cash address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:   xc.NativeAsset(tt.chain),
				Network: tt.network,
			}

			err := bitcoin_cash.ValidateAddress(cfg, tt.address)

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
