package builder

import (
	"errors"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	tx "github.com/cordialsys/crosschain/chain/internet_computer/tx"
	tx_input "github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	transaction, err := tx.NewTx(args, *txInput)
	return &transaction, err
}
