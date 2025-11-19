package newchain_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	newchain "github.com/cordialsys/crosschain/chain/template"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	// This is a template test - replace with actual test cases for your chain
	tests := []struct {
		name      string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Template - not implemented",
			address:   "any_address",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.NativeAsset("TEMPLATE"),
			}

			err := newchain.ValidateAddress(cfg, tt.address)

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
