package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/stretchr/testify/require"
)

func TestMultiTransferRejectsTokenReceivers(t *testing.T) {
	chain := xc.NewChainConfig(xc.BTC).WithNet("testnet")
	b, err := builder.NewTxBuilder(chain.Base())
	require.NoError(t, err)
	address := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
	for _, tc := range []struct {
		name     string
		index    int
		contract xc.ContractAddress
	}{
		{"first receiver", 0, "unsupported-token"},
		{"middle receiver", 1, "unsupported-token"},
		{"last receiver", 2, "unsupported-token"},
		{"explicit empty contract", 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receivers := make([]*xcbuilder.Receiver, 3)
			for i := range receivers {
				var options []xcbuilder.BuilderOption
				if i == tc.index {
					options = append(options, xcbuilder.OptionContractAddress(tc.contract, 6))
				}
				receivers[i] = buildertest.MustNewReceiver(address, xc.NewAmountBlockchainFromUint64(1_000_000), options...)
			}
			args, err := xcbuilder.NewMultiTransferArgs(chain.Base(),
				[]*xcbuilder.Sender{buildertest.MustNewSender(address, nil)}, receivers)
			require.NoError(t, err)
			// Reject unsupported assets before accessing inputs or producing any outputs.
			var input *tx_input.MultiTransferInput
			transaction, err := b.MultiTransfer(*args, input)
			require.EqualError(t, err, "token transfers are not supported on BTC")
			require.Nil(t, transaction)
		})
	}
}
