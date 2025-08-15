package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/dusk/builder"
	"github.com/cordialsys/crosschain/chain/dusk/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	builder1, err := builder.NewTxBuilder(xc.NewChainConfig(xc.DUSK).Base())
	require.NotNil(t, builder1)
	require.NoError(t, err)
}

func TestNewTransfer(t *testing.T) {
	builder, _ := builder.NewTxBuilder(xc.NewChainConfig(xc.DUSK).Base())
	from := xc.Address("2293LeWtYGpsBA99HRg2AfMm9oYhikZ83GSW5NP6QtQxDvkBTAdU8LfQj9fXvDt1rK1baqBcf3gQKsLXpw3LUjpdkSMRMrTsfuTo5Yri1xvUDnVcMMpgTG4o7ThCjZuLMp9L")
	to := xc.Address("26nbWp93it1FF8ChyBUmV2zrXMqsv6xR41UUfcyq37abhoYvvEW4C8MgJPdKnzfQhfa6t1VtVj2QUeDK1aP98TGGtumV897Gtv3M7mh2qZBNK6C4LqvP6GyTeHvC7kPncVvg")
	amount := xc.NewAmountBlockchainFromStr("1")

	input := &TxInput{
		Nonce:    1,
		GasLimit: 250000,
		GasPrice: 1,
		ChainId:  1,
	}
	asset := xc.NewChainConfig(xc.DUSK)

	tf, err := builder.Transfer(buildertest.MustNewTransferArgs(asset.Base(), from, to, amount), input)
	require.NoError(t, err)
	require.NotNil(t, tf)
}
