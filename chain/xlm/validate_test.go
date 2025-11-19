package xlm_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Stellar addresses
		{
			name:      "XLM - valid address 1",
			address:   "GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4",
			wantError: false,
		},
		{
			name:      "XLM - valid address 2",
			address:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			wantError: false,
		},
		{
			name:      "XLM - valid muxed address (M prefix)",
			address:   "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAABUTGI4",
			wantError: false,
		},
		// Invalid addresses
		{
			name:      "XLM - missing G prefix",
			address:   "ACTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "XLM - wrong prefix (M)",
			address:   "MCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "XLM - wrong prefix (T)",
			address:   "TCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
		{
			name:      "XLM - too short (55 chars)",
			address:   "GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ",
			wantError: true,
			errorMsg:  "must be 56 characters",
		},
		{
			name:      "XLM - too long (57 chars)",
			address:   "GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ4A",
			wantError: true,
			errorMsg:  "must be 56 characters",
		},
		{
			name:      "XLM - invalid base32 character (0)",
			address:   "GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJ0Z",
			wantError: true,
			errorMsg:  "invalid base32 encoding",
		},
		{
			name:      "XLM - invalid checksum",
			address:   "GCTUKHQ7655O6ZT3OQ3QTDTQSD6KUJCTHTN2YYTHNG5WWXWGW7MUYJZ5",
			wantError: true,
			errorMsg:  "checksum mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.XLM,
			}

			err := xlm.ValidateAddress(cfg, tt.address)

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
