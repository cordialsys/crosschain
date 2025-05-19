package builder

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/kaspa/tx"
	"github.com/cordialsys/crosschain/chain/kaspa/tx_input"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	prefixInt, _ := txBuilder.Asset.ChainPrefix.AsInt()
	tx := tx.NewTx(args, txInput, prefixInt)
	return tx, nil
}
