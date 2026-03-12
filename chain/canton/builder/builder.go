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
	if contract, ok := args.GetContract(); ok && string(contract) != string(txBuilder.Asset.Chain) {
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

	if cantonInput.ContractAddress != "" {
		decimals := int(cantonInput.Decimals)
		if decimals <= 0 {
			decimals = int(txBuilder.Asset.Decimals)
		}
		tokenArgs, err := xcbuilder.NewTransferArgs(
			txBuilder.Asset,
			args.GetFrom(),
			args.GetTo(),
			args.GetAmount(),
			xcbuilder.OptionContractAddress(cantonInput.ContractAddress, decimals),
		)
		if err != nil {
			return nil, err
		}
		args = tokenArgs
	}

	decimals := cantonInput.Decimals
	if decimals <= 0 {
		decimals = txBuilder.Asset.Decimals
	}
	return cantontx.NewTx(cantonInput, args, decimals)
}

func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	cantonInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid tx input type for Canton, expected *tx_input.TxInput")
	}

	decimals := cantonInput.Decimals
	if decimals <= 0 {
		decimals = txBuilder.Asset.Decimals
	}
	return cantontx.NewTx(cantonInput, args, decimals)
}

func (txBuilder TxBuilder) CreateAccount(args xcbuilder.CreateAccountArgs, input xc.CreateAccountTxInput) (xc.Tx, error) {
	return cantontx.NewCreateAccountTx(args, input)
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	return xc.MemoSupportString
}
