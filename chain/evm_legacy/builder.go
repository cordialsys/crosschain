package evm_legacy

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/core/types"
)

var DefaultMaxTipCapGwei uint64 = 5

// TxBuilder for EVM
type TxBuilder evmbuilder.TxBuilder

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xc.TxTokenBuilder = &TxBuilder{}
var _ xc.TxXTransferBuilder = &TxBuilder{}

// NewTxBuilder creates a new EVM TxBuilder
func NewTxBuilder(asset xc.ITask) (TxBuilder, error) {
	builder, err := evmbuilder.NewTxBuilder(asset)
	if err != nil {
		return TxBuilder{}, err
	}
	builder = builder.WithTxBuilder(&LegacyEvmTxBuilder{})

	return TxBuilder(builder), nil
}

// supports evm before london merge
type LegacyEvmTxBuilder struct {
}

var _ evmbuilder.GethTxBuilder = &LegacyEvmTxBuilder{}

func parseInput(input xc.TxInput) (*TxInput, error) {
	switch input := input.(type) {
	case *TxInput:
		return input, nil
	case *evminput.TxInput:
		return (*TxInput)(input), nil
	default:
		return nil, fmt.Errorf("invalid input type %T", input)
	}
}

func (*LegacyEvmTxBuilder) BuildTxWithPayload(chain *xc.ChainConfig, to xc.Address, value xc.AmountBlockchain, data []byte, inputRaw xc.TxInput) (xc.Tx, error) {
	address, err := evmaddress.FromHex(to)
	if err != nil {
		return nil, err
	}
	chainID := new(big.Int).SetInt64(chain.ChainID)
	input, err := parseInput(inputRaw)
	if err != nil {
		return nil, err
	}
	// Protection from setting very high gas tip
	// TODO

	return &Tx{
		EthTx: types.NewTransaction(
			input.Nonce,
			address,
			value.Int(),
			input.GasLimit,
			input.GasPrice.Int(),
			data,
		),
		Signer: types.LatestSignerForChainID(chainID),
	}, nil
}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	// type cast back to evm input, which is expected by the evm builder
	inputEvm := (*evminput.TxInput)(input.(*TxInput))
	return evmbuilder.TxBuilder(txBuilder).NewTransfer(from, to, amount, inputEvm)
}

func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	inputEvm := (*evminput.TxInput)(input.(*TxInput))
	return evmbuilder.TxBuilder(txBuilder).NewNativeTransfer(from, to, amount, inputEvm)
}

func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	inputEvm := (*evminput.TxInput)(input.(*TxInput))
	return evmbuilder.TxBuilder(txBuilder).NewTokenTransfer(from, to, amount, inputEvm)
}

func (txBuilder TxBuilder) NewTask(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	inputEvm := (*evminput.TxInput)(input.(*TxInput))
	return evmbuilder.TxBuilder(txBuilder).NewTask(from, to, amount, inputEvm)
}
