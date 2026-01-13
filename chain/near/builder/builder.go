package builder

import (
	"errors"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/near/tx"
	near_input "github.com/cordialsys/crosschain/chain/near/tx_input"
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
	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args, contract, input)
	} else {
		return txBuilder.NewNativeTransfer(args, input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	nearInput, ok := input.(*near_input.TxInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	return tx.NewNativeTx(nearInput, args)
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	nearInput, ok := input.(*near_input.TxInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	return tx.NewTokenTx(nearInput, args)
}
