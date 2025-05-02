package builder

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	tx "github.com/cordialsys/crosschain/chain/cardano/tx"
	cardanoinput "github.com/cordialsys/crosschain/chain/cardano/tx_input"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

type TxInput = cardanoinput.TxInput

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args, contract, input)
	} else {
		return txBuilder.NewNativeTransfer(args, input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput, ok := input.(*TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	return tx.NewTx(args, *txInput)
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, errors.New("not implemented")
}
