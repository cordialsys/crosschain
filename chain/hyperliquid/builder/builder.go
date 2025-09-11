package builder

import (
	"errors"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
)

// TxBuilder for hyperliquid
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new hyperliquid TxBuilder
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

	transaction := tx.NewTx(args, *txInput)
	return &transaction, nil
}
