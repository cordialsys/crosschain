package kaspa_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/kaspa"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name        string
		chainPrefix string
		address     xc.Address
		wantError   bool
		errorMsg    string
	}{
		// Valid Kaspa addresses
		{
			name:        "Kaspa - valid mainnet address",
			chainPrefix: "kaspa",
			address:     "kaspa:qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9qng8tfmwmf8",
			wantError:   false,
		},
		{
			name:        "Kaspa - valid devnet address",
			chainPrefix: "kaspadev",
			address:     "kaspadev:qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9qngt4vauxut",
			wantError:   false,
		},
		// Invalid addresses
		{
			name:        "Kaspa - missing prefix separator",
			chainPrefix: "kaspa",
			address:     "kaspa_qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9qng8tfmwmf8",
			wantError:   true,
			errorMsg:    "invalid index of ':'",
		},
		{
			name:        "Kaspa - invalid bech32 encoding",
			chainPrefix: "kaspa",
			address:     "kaspa:invalid-bech32-data!!!",
			wantError:   true,
			errorMsg:    "invalid kaspa address",
		},
		{
			name:        "Kaspa - too short",
			chainPrefix: "kaspa",
			address:     "kaspa:qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9q",
			wantError:   true,
			errorMsg:    "invalid kaspa address",
		},
		{
			name:        "Kaspa - invalid checksum",
			chainPrefix: "kaspa",
			address:     "kaspa:qr5ln3lfh8y323x6a2le5nraj7p334wezjc5ctt0kd6phlype9qng8tfmwmf9",
			wantError:   true,
			errorMsg:    "invalid kaspa address",
		},
		{
			name:        "Kaspa - bitcoin address",
			chainPrefix: "kaspa",
			address:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantError:   true,
			errorMsg:    "invalid kaspa address",
		},
		{
			name:        "Kaspa - cosmos address",
			chainPrefix: "kaspa",
			address:     "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			wantError:   true,
			errorMsg:    "invalid index of ':'",
		},
		{
			name:        "Kaspa - ethereum address",
			chainPrefix: "kaspa",
			address:     "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
			wantError:   true,
			errorMsg:    "invalid kaspa address",
		},
		{
			name:        "Kaspa - EOS address",
			chainPrefix: "kaspa",
			address:     "EOS6bjiYSr66ZxpRDoZpFchhuGGP6SFNrzLyNM234TkeSNfWN2C1s",
			wantError:   true,
			errorMsg:    "invalid kaspa address",
		},
		{
			name:        "Kaspa - filecoin address",
			chainPrefix: "kaspa",
			address:     "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
			wantError:   true,
			errorMsg:    "invalid index of ':'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain:       xc.KAS,
				ChainPrefix: xc.StringOrInt(tt.chainPrefix),
			}

			err := kaspa.ValidateAddress(cfg, tt.address)

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
