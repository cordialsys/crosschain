package evm_legacy

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
)

var DefaultMaxTipCapGwei uint64 = 5

// TxBuilder for EVM legacy
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

// NewTxBuilder creates a new EVM TxBuilder
func NewTxBuilder(asset *xc.ChainBaseConfig) (TxBuilder, error) {
	builder, err := evmbuilder.NewTxBuilder(asset)
	if err != nil {
		return TxBuilder{}, err
	}

	return TxBuilder(builder), nil
}

func parseInput(input xc.TxInput) (*evminput.TxInput, error) {
	switch input := input.(type) {
	case *evminput.TxInput:
		return input, nil
	case *TxInput:
		return (*evminput.TxInput)(input), nil
	default:
		return nil, fmt.Errorf("invalid input type %T", input)
	}
}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	casted, err := parseInput(input)
	if err != nil {
		return nil, err
	}
	return NewTx(txBuilder.Asset, args, casted, true)
}
