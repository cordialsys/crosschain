package near_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/near"
	"github.com/cordialsys/crosschain/factory/config"
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
			name:      "valid implicit address",
			address:   "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			wantError: false,
		},
		{
			name:      "invalid hex character",
			address:   "o6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			wantError: true,
		},
		{
			name:      "implicit too long",
			address:   "dd6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			wantError: true,
		},
		{
			name:      "valid explicit",
			address:   "crosschain.near",
			wantError: false,
		},
		{
			name:      "explicit address invalid network",
			address:   "crosschain.asdf",
			wantError: true,
		},
		{
			name:      "explicit address invalid character",
			address:   "crosschAin.asdf",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:   xc.NEAR,
				Network: string(config.Mainnet),
			}

			err := near.ValidateAddress(cfg, tt.address)

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
