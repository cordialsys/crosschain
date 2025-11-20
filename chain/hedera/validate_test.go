package hedera_test

import (
	"fmt"
	"math"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/hedera"
	"github.com/stretchr/testify/require"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		network   string
		address   xc.Address
		wantError bool
		errorMsg  string
	}{
		// Valid Hedera account ids
		{
			name:      "Hedera - min account id",
			address:   "0.0.1",
			wantError: false,
		},
		{
			name:      "Hedera - max account id",
			address:   xc.Address(fmt.Sprintf("0.0.%d", math.MaxInt64)),
			wantError: false,
		},
		{
			name:      "Hedera - max realm num",
			address:   xc.Address(fmt.Sprintf("0.%d.0", math.MaxInt64)),
			wantError: false,
		},
		{
			name:      "Hedera - max shard num",
			address:   xc.Address(fmt.Sprintf("%d.0.0", math.MaxInt64)),
			wantError: false,
		},
		// Invalid hedera acccounts
		{
			name:      "Hedera - overflow account id",
			address:   xc.Address(fmt.Sprintf("0.0.%d", uint64(math.MaxInt64+1))),
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "Hedera - overflow realm num",
			address:   xc.Address(fmt.Sprintf("0.%d.0", uint64(math.MaxInt64+1))),
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "Hedera - overflow shard num",
			address:   xc.Address(fmt.Sprintf("%d.0.0", uint64(math.MaxInt64+1))),
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "Hedera - space in the middle",
			address:   "0.0 .0",
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "Hedera - invalid character",
			address:   "0.0i.0",
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "Hedera - line break",
			address:   "0.0\n.0",
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "Hedera - hex acc num",
			address:   "0.0.aa",
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		// Valid EVM addresses
		{
			name:      "EVM - valid address (lowercase)",
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fce1",
			wantError: false,
		},
		{
			name:      "EVM - valid XDC address (mixed case)",
			address:   "xdc5891906fEf64A5ae924C7Fc5ed48c0F64a55fCe1",
			wantError: true,
		},
		{
			name:      "EVM - space in address",
			address:   "0x5891906fef64a5ae92 c7fc5ed48c0f64a55fce1",
			wantError: true,
			errorMsg:  "invalid evm address",
		},
		{
			name:      "EVM - wrong prefix",
			address:   "0y5891906fef64a5ae924c7fc5ed48c0f64a55fce1",
			wantError: true,
			errorMsg:  "invalid hedera account id",
		},
		{
			name:      "EVM - too short (39 chars)",
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fce",
			wantError: true,
			errorMsg:  "invalid evm address",
		},
		{
			name:      "EVM - too long (41 chars)",
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fce11",
			wantError: true,
			errorMsg:  "invalid evm address",
		},
		{
			name:      "EVM - invalid hex characters",
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fceg",
			wantError: true,
			errorMsg:  "invalid evm address",
		},
		{
			name:      "EVM - invalid hex characters (z)",
			address:   "0x5891906fef64a5ae924c7fc5ed48c0f64a55fcez",
			wantError: true,
			errorMsg:  "invalid evm address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cfg := &xc.ChainBaseConfig{
				Chain: xc.NativeAsset(xc.HBAR),
			}

			err := hedera.ValidateAddress(cfg, tt.address)

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
