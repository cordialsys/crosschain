package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
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
		pubkey          string
		expectedAddress string
		error           string
	}{
		{
			name:            "PreprodNetwork",
			network:         "preprod",
			pubkey:          "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			expectedAddress: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
		},
		{
			name:    "StagingNetwork",
			network: "staging",
			pubkey:  "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			// Address HMR differs only for mainnet
			expectedAddress: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
		},
		{
			name:            "MainnetNetwork",
			network:         "mainnet",
			pubkey:          "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c",
			expectedAddress: "addr1vxjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s02dyt3",
		},
		{
			name:            "PubkeyTooLong",
			network:         "mainnet",
			pubkey:          "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72caaaaaa",
			expectedAddress: "addr1vxjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s02dyt3",
			error:           "invalid public key length",
		},
	}

	for _, v := range vetors {
		t.Run(v.name, func(t *testing.T) {
			cfg := xc.NewChainConfig(xc.ADA).WithNet(v.network)
			builder, err := address.NewAddressBuilder(cfg.Base())
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
