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
	Asset xc.ITask
}

type TxInput = filinput.TxInput

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new filecoin TxBuilder
func NewTxBuilder(cfgI xc.ITask) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

// Old transfer interface
func (txBuilder TxBuilder) NewTransfer(xcFrom xc.Address, xcTo xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	transferArgs, err := xcbuilder.NewTransferArgs(xcFrom, xcTo, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer args: %w", err)
	}
	txInput, ok := input.(*TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	return &tx.Tx{
		Message: tx.NewMessage(transferArgs, *txInput),
	}, nil
}
