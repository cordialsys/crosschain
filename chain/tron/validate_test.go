package tron_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/tron"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid TRON addresses
		{
			name:      "TRON - valid address 1",
			address:   "TDpBe64DqirkKWj6HWuR1pWgmnhw2wDacE",
			wantError: false,
		},
		{
			name:      "TRON - valid address 2",
			address:   "TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5",
			wantError: false,
		},
		{
			name:      "TRON - valid address 3",
			address:   "TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8",
			wantError: false,
		},
		{
			name:      "TRON - valid address 4",
			address:   "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs",
			wantError: false,
		},
		{
			name:      "TRON - valid address 5",
			address:   "TJmka325yjJKeFpQDwKSQAoNwEyNGhsaEV",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "TRON - missing T prefix",
			address:   "DpBe64DqirkKWj6HWuR1pWgmnhw2wDacE1",
			wantError: true,
			errorMsg:  "invalid base58check encoding",
		},
		{
			name:      "TRON - wrong prefix (1)",
			address:   "1DpBe64DqirkKWj6HWuR1pWgmnhw2wDacE",
			wantError: true,
			errorMsg:  "invalid base58check encoding",
		},
		{
			name:      "TRON - wrong prefix (0x)",
			address:   "0xDpBe64DqirkKWj6HWuR1pWgmnhw2wDacE",
			wantError: true,
			errorMsg:  "invalid base58check encoding",
		},
		{
			name:      "TRON - too short",
			address:   "TDpBe64DqirkKWj6HWuR1pWgmnhw2wDac",
			wantError: true,
			errorMsg:  "checksum error",
		},
		{
			name:      "TRON - too long",
			address:   "TDpBe64DqirkKWj6HWuR1pWgmnhw2wDacE1",
			wantError: true,
			errorMsg:  "checksum error",
		},
		{
			name:      "TRON - invalid base58 characters",
			address:   "TDpBe64DqirkKWj6HWuR1pWgmnhw2wDa0O",
			wantError: true,
			errorMsg:  "invalid base58check encoding",
		},
		{
			name:      "TRON - invalid checksum",
			address:   "TDpBe64DqirkKWj6HWuR1pWgmnhw2wDacF",
			wantError: true,
			errorMsg:  "invalid base58check encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.TRX,
			}

			err := tron.ValidateAddress(cfg, tt.address)

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
