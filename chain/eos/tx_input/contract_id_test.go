package tx_input

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func TestParseContractId(t *testing.T) {
	tests := []struct {
		name         string
		chain        *xc.ChainBaseConfig
		contractId   xc.ContractAddress
		input        *TxInput
		wantContract string
		wantSymbol   string
		err          string
	}{
		{
			name:         "contract with slash separator",
			chain:        &xc.ChainBaseConfig{},
			contractId:   "eosio.token/EOS",
			input:        nil,
			wantContract: "eosio.token",
			wantSymbol:   "EOS",
		},
		{
			name:         "contract with dash separator",
			chain:        &xc.ChainBaseConfig{},
			contractId:   "core.vaulta-A",
			input:        nil,
			wantContract: "core.vaulta",
			wantSymbol:   "A",
		},
		{
			name:         "contract with symbol from input",
			chain:        &xc.ChainBaseConfig{},
			contractId:   "eosio.token",
			input:        &TxInput{Symbol: "EOS"},
			wantContract: "eosio.token",
			wantSymbol:   "EOS",
		},
		{
			name: "contract with native asset alias match",
			chain: &xc.ChainBaseConfig{
				NativeAssets: []*xc.AdditionalNativeAsset{
					{
						ContractId: "vaulta.token/A",
						Aliases:    []string{"vaulta.token"},
					},
				},
			},
			contractId:   "vaulta.token",
			input:        nil,
			wantContract: "vaulta.token",
			wantSymbol:   "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContract, gotSymbol, err := ParseContractId(tt.chain, tt.contractId, tt.input)

			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantContract, gotContract, "contract")
				require.Equal(t, tt.wantSymbol, gotSymbol, "symbol")
			}

		})
	}
}
