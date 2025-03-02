package tron_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/stretchr/testify/require"
)

// func TestDeserialiseTransactionEvents(t *testing.T) {
// 	dummyEvent := new(core.TransactionInfo_Log)
// 	dummyEvent.Address, _ = address.Base58ToAddress("TKHN9ED3N4psUbKs6sCGKPuoLxFWdb3Ud5")

// 	dummyEvent.Topics = make([][]byte, 3)

// 	hex.Decode(dummyEvent.Topics[0], []byte("41b0ae4259cc90928d4a57d341f09a1fd28347867a"))
// 	hex.Decode(dummyEvent.Topics[1], []byte("41b737f97351c2a20fd7de320221a774c3ca837b94"))
// 	hex.Decode(dummyEvent.Data, []byte("0x000000000000000000000000000000000000000000000000000000000015f900"))
// }

// Since no estimation is used currently, relies on client configuration
func TestTronChainsHaveDefaultGasBudget(t *testing.T) {
	require := require.New(t)
	configs := []*factory.Factory{
		factory.NewFactory(&factory.FactoryOptions{}),
		factory.NewNotMainnetsFactory(&factory.FactoryOptions{}),
	}
	for _, cfg := range configs {
		for _, chain := range cfg.GetAllChains() {
			if chain.Chain.Driver() == xc.DriverTron {
				require.NotNil(chain.GasBudgetDefault, "TRON chains must have a default gas budget")
				require.NotZero(
					chain.GasBudgetDefault.ToBlockchain(0).Uint64(), "TRON chains must have a default gas budget")
			}
		}
	}
}
