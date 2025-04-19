package aptos

import (
	"errors"

	transactionbuilder "github.com/coming-chat/go-aptos/transaction_builder"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}
var _ xcbuilder.BuilderSupportsFeePayer = TxBuilder{}

func (txBuilder TxBuilder) SupportsFeePayer() {
}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(asset *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: asset,
	}, nil
}

// Transfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	var local_input *tx_input.TxInput
	var ok bool
	if local_input, ok = (input.(*tx_input.TxInput)); !ok {
		return &Tx{}, errors.New("xc.TxInput is not from an aptos chain")
	}

	feePayer, ok := args.GetFeePayer()
	if !ok {
		feePayer = args.GetFrom()
	}

	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(feePayer, args.GetFrom(), args.GetTo(), args.GetAmount(), contract, local_input)
	} else {
		return txBuilder.NewNativeTransfer(feePayer, args.GetFrom(), args.GetTo(), args.GetAmount(), local_input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(feePayer xc.Address, from xc.Address, to xc.Address, amount xc.AmountBlockchain, input *tx_input.TxInput) (xc.Tx, error) {

	from_addr, err := DecodeAddress(string(from))
	if err != nil {
		return &Tx{}, err
	}

	to_addr, err := DecodeAddress(string(to))
	if err != nil {
		return &Tx{}, err
	}
	toAmountBytes := transactionbuilder.BCSSerializeBasicValue(amount.Int().Uint64())

	chain_id := input.ChainId
	moduleName, err := transactionbuilder.NewModuleIdFromString("0x1::aptos_account")
	if err != nil {
		return &Tx{}, err
	}
	payload := transactionbuilder.TransactionPayloadEntryFunction{
		ModuleName:   *moduleName,
		FunctionName: "transfer",
		Args: [][]byte{
			to_addr[:], toAmountBytes,
		},
	}

	tx := &Tx{
		rawTx: transactionbuilder.RawTransaction{
			Sender:         from_addr,
			SequenceNumber: input.SequenceNumber,
			Payload:        payload,
			MaxGasAmount:   input.GasLimit,
			GasUnitPrice:   input.GasPrice,
			// ~1 hour expiration
			ExpirationTimestampSecs: input.Timestamp + 60*60,
			ChainId:                 uint8(chain_id),
		},
		Input: input,
	}

	if feePayer != from {
		tx.extraFeePayer = feePayer
	}
	return tx, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txb *TxBuilder) NewTokenTransfer(feePayer xc.Address, from xc.Address, to xc.Address, amount xc.AmountBlockchain, contract xc.ContractAddress, input *tx_input.TxInput) (xc.Tx, error) {

	from_addr, err := DecodeAddress(string(from))
	if err != nil {
		return &Tx{}, err
	}
	to_addr, err := DecodeAddress(string(to))
	if err != nil {
		return &Tx{}, err
	}

	toAmountBytes := transactionbuilder.BCSSerializeBasicValue(amount.Int().Uint64())
	typeTag, err := transactionbuilder.NewTypeTagStructFromString(string(contract))
	if err != nil {
		return nil, err
	}

	chain_id := input.ChainId
	moduleName, err := transactionbuilder.NewModuleIdFromString("0x1::coin")
	if err != nil {
		return &Tx{}, err
	}
	payload := transactionbuilder.TransactionPayloadEntryFunction{
		ModuleName:   *moduleName,
		FunctionName: "transfer",
		TyArgs:       []transactionbuilder.TypeTag{*typeTag},
		Args: [][]byte{
			to_addr[:], toAmountBytes,
		},
	}
	tx := &Tx{
		rawTx: transactionbuilder.RawTransaction{
			Sender:         from_addr,
			SequenceNumber: input.SequenceNumber,
			Payload:        payload,
			MaxGasAmount:   input.GasLimit,
			GasUnitPrice:   input.GasPrice,
			// ~1 hour expiration
			ExpirationTimestampSecs: input.Timestamp + 60*60,
			ChainId:                 uint8(chain_id),
		},
		Input: input,
	}
	if feePayer != from {
		tx.extraFeePayer = feePayer
	}
	return tx, nil
}
