package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	tx "github.com/cordialsys/crosschain/chain/filecoin/tx"
	filinput "github.com/cordialsys/crosschain/chain/filecoin/tx_input"
)

// TxBuilder for filecoin
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

type TxInput = filinput.TxInput

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new filecoin TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput, ok := input.(*TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	return &tx.Tx{
		Message: tx.NewMessage(args, *txInput),
	}, nil
}
