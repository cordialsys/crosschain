package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/chain/cardano/address"
	"github.com/stretchr/testify/require"
)

func TestMainnetAddressBuilder(t *testing.T) {
	cfg := xc.NewChainConfig(xc.ADA).WithNet("mainnet")
	builder, err := address.NewAddressBuilder(cfg.Base())

	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Equal(t, true, builder.(address.AddressBuilder).IsMainnet)
}

func TestPreprodAddressBuilder(t *testing.T) {
	cfg := xc.NewChainConfig(xc.ADA).WithNet("preprod")
	builder, err := address.NewAddressBuilder(cfg.Base())

	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Equal(t, false, builder.(address.AddressBuilder).IsMainnet)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	vetors := []struct {
		name            string
		network         string
		format          string
		pubkey          string
		expectedAddress string
		error           string
	}{
		{
			name:            "PreprodNetwork",
			network:         "preprod",
			format:          "legacy",
			pubkey:          "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			expectedAddress: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
		},
		{
			name:    "StagingNetwork",
			network: "staging",
			format:  "legacy",
			pubkey:  "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			// Address HMR differs only for mainnet
			expectedAddress: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
		},
		{
			name:            "MainnetNetwork",
			network:         "mainnet",
			format:          "legacy",
			pubkey:          "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			expectedAddress: "addr1vxjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s02dyt3",
		},
		{
			name:    "PubkeyTooLong",
			network: "mainnet",
			pubkey:  "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72caaaaaa",
			error:   "invalid public key length",
		},
		{
			name:            "PaymentAddress",
			network:         "preprod",
			format:          "payment",
			pubkey:          "f0bb6fd00a035b6b6ec18bbb2739265b80f319c0634333fe678928f40750cade",
			expectedAddress: "addr_test1qpp33pdh0nppmxz8ma2def28nz8kju0yqrnmgfelcjf88fzrrzzmwlxzrkvy0h65mjj50xy0d9c7gq88ksnnl3yjwwjqxdkju2",
		},
		{
			name:            "LegacyDiffersFromPayment",
			network:         "preprod",
			format:          "legacy",
			pubkey:          "f0bb6fd00a035b6b6ec18bbb2739265b80f319c0634333fe678928f40750cade",
			expectedAddress: "addr_test1vpp33pdh0nppmxz8ma2def28nz8kju0yqrnmgfelcjf88fqda3z6z",
		},
		{
			name:            "StakeAddress",
			network:         "preprod",
			format:          "stake",
			pubkey:          "f0bb6fd00a035b6b6ec18bbb2739265b80f319c0634333fe678928f40750cade",
			expectedAddress: "stake_test1upp33pdh0nppmxz8ma2def28nz8kju0yqrnmgfelcjf88fqd406dg",
		},
	}

	for _, v := range vetors {
		t.Run(v.name, func(t *testing.T) {
			cfg := xc.NewChainConfig(xc.ADA).WithNet(v.network)
			builder, err := address.NewAddressBuilder(cfg.Base(), xcaddress.OptionFormat(xc.AddressFormat(v.format)))
			require.NoError(t, err)

			pubkeyBytes, err := hex.DecodeString(v.pubkey)
			address, err := builder.GetAddressFromPublicKey(pubkeyBytes)
			if v.error == "" {
				require.NoError(t, err)
				require.Equal(t, v.expectedAddress, string(address))
			} else {
				require.ErrorContains(t, err, v.error)
			}
		})
	}
}
