package internet_computer_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/internet_computer"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid ICP addresses (hex format)
		{
			name:      "ICP - valid hex address",
			address:   "723b365d1ca4bd14f4899cb3d6d028434e3219a9f4de98fa52553dfb6c363af4",
			wantError: false,
		},
		{
			name:      "ICP - valid hex address (uppercase)",
			address:   "6C5066261553064A8D4FA8F30FA9D587D9887BCE69601CDB5B6CAC8780FC8899",
			wantError: false,
		},
		{
			name:      "ICP - valid hex address (mixed case)",
			address:   "6c5066261553064A8d4fa8f30fa9d587D9887bce69601cdb5b6cac8780fc8899",
			wantError: false,
		},
		// Valid ICRC1 addresses (base32 with dashes)
		{
			name:      "ICRC1 - valid address 1",
			address:   "mglk4-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-zae",
			wantError: false,
		},
		{
			name:      "ICRC1 - valid address 2",
			address:   "ehq2s-mlmrx-xvogi-adg5n-dowp3-qasvb-nvdjl-ijj3m-e6ijc-pmrmb-dqe",
			wantError: false,
		},
		{
			name:      "ICRC1 - valid short address",
			address:   "aaaaa-aa",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "ICP - too short (31 bytes)",
			address:   "6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc88",
			wantError: true,
			errorMsg:  "must be 32 bytes",
		},
		{
			name:      "ICP - too long (33 bytes)",
			address:   "6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc889900",
			wantError: true,
			errorMsg:  "must be 32 bytes",
		},
		{
			name:      "ICP - invalid hex characters",
			address:   "6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc88zz",
			wantError: true,
			errorMsg:  "unknown format",
		},
		{
			name:      "ICRC1 - invalid segment length (too long)",
			address:   "mglk42-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-zae",
			wantError: true,
			errorMsg:  "invalid length",
		},
		{
			name:      "ICRC1 - invalid base32 encoding",
			address:   "mglk4-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-za1",
			wantError: true,
			errorMsg:  "illegal base32 data",
		},
		{
			name:      "ICP - bitcoin address",
			address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError: true,
			errorMsg:  "unknown format",
		},
		{
			name:      "ICP - cosmos address",
			address:   "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError: true,
			errorMsg:  "unknown format",
		},
		{
			name:      "ICP - ethereum address",
			address:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError: true,
			errorMsg:  "unknown format",
		},
		{
			name:      "ICP - EOS address",
			address:   "EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError: true,
			errorMsg:  "unknown format",
		},
		{
			name:      "ICP - filecoin address",
			address:   "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			wantError: true,
			errorMsg:  "unknown format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.ICP,
			}

			err := internet_computer.ValidateAddress(cfg, tt.address)

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
