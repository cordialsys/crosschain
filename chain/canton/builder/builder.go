package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
)

// TxBuilder for Canton
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}
var _ xcbuilder.AccountCreation = TxBuilder{}

// NewTxBuilder creates a new Canton TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// Transfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args, contract, input)
	}
	return txBuilder.NewNativeTransfer(args, input)
}

// NewNativeTransfer creates a Tx from the prepared transaction in TxInput.
// The heavy lifting (command building, prepare call) was done in FetchTransferInput.
func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	cantonInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid tx input type for Canton, expected *tx_input.TxInput")
	}

	return cantontx.NewTx(cantonInput, args, txBuilder.Asset.Decimals)
}

// NewTokenTransfer is not supported for Canton
func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("token transfers are not supported for %s", txBuilder.Asset.Chain)
}

func (txBuilder TxBuilder) CreateAccount(args xcbuilder.CreateAccountArgs, input xc.CreateAccountTxInput) (xc.Tx, error) {
	return cantontx.NewCreateAccountTx(args, input)
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	return xc.MemoSupportNone
}
