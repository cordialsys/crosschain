package builder_test

import (
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/hyperliquid/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	chainCfg := xc.NewChainConfig("HYPE").Base()
	builder1, err := builder.NewTxBuilder(chainCfg)
	require.NotNil(t, builder1)
	require.NoError(t, err)
}

func TestNewTransfer(t *testing.T) {
	chainCfg := xc.NewChainConfig("HYPE").Base()
	builder, err := builder.NewTxBuilder(chainCfg)
	require.NoError(t, err)

	from := xc.Address("0xdb32f3f4c2ec447c7e15dd5df45055c08652f4db")
	to := xc.Address("0x21db009054831a7fd8914f544f749180630ce217")
	amount := xc.NewAmountBlockchainFromStr("0.1")

	input := &TxInput{
		TxInputEnvelope:  xc.TxInputEnvelope{},
		TransactionTime:  time.UnixMilli(0),
		Decimals:         0,
		Token:            "",
		HyperliquidChain: "",
	}
	args := buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
	)

	_, err = builder.Transfer(args, input)
	require.NoError(t, err)
}
