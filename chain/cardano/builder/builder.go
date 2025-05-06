package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	tx "github.com/cordialsys/crosschain/chain/cardano/tx"
	cardanoinput "github.com/cordialsys/crosschain/chain/cardano/tx_input"
)

// Cardano TxBuilder
type TxBuilder struct{}

type TxInput = cardanoinput.TxInput

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput, ok := input.(*TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	return tx.NewTx(args, *txInput)
}
