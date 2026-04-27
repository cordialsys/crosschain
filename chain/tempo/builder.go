package tempo

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
)

type TxBuilder struct {
	evmbuilder.TxBuilder
}

func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) {
	evmBuilder, err := evmbuilder.NewTxBuilder(cfg)
	if err != nil {
		return TxBuilder{}, err
	}

	return TxBuilder{
		TxBuilder: evmBuilder,
	}, nil
}

var _ xcbuilder.FullBuilder = &TxBuilder{}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	evmInput, err := evmTransferInput(input)
	if err != nil {
		return nil, err
	}
	return evmtx.NewTx(txBuilder.Asset, args, evmInput, false)
}

func (txBuilder TxBuilder) MultiTransfer(args xcbuilder.MultiTransferArgs, input xc.MultiTransferInput) (xc.Tx, error) {
	switch input := input.(type) {
	case *MultiTransferInput:
		return evmtx.NewMultiTx(txBuilder.Asset, args, &input.TxInput.TxInput)
	case *evminput.MultiTransferInput:
		return evmtx.NewMultiTx(txBuilder.Asset, args, &input.TxInput)
	default:
		return nil, fmt.Errorf("unsupported Tempo multi-transfer input type %T", input)
	}
}

func evmTransferInput(input xc.TxInput) (*evminput.TxInput, error) {
	switch input := input.(type) {
	case *TxInput:
		return &input.TxInput, nil
	case *evminput.TxInput:
		return input, nil
	default:
		return nil, fmt.Errorf("unsupported Tempo transfer input type %T", input)
	}
}
