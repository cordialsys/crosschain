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
	Asset xc.ITask
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(asset xc.ITask) (TxBuilder, error) {
	return TxBuilder{
		Asset: asset,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	var local_input *tx_input.TxInput
	var ok bool
	if local_input, ok = (input.(*tx_input.TxInput)); !ok {
		return &Tx{}, errors.New("xc.TxInput is not from an aptos chain")
	}

	if _, ok := txBuilder.Asset.(*xc.TokenAssetConfig); ok {
		return txBuilder.NewTokenTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), local_input)
	}
	return txBuilder.NewNativeTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), local_input)
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input *tx_input.TxInput) (xc.Tx, error) {

	to_addr := [transactionbuilder.ADDRESS_LENGTH]byte{}
	from_addr := [transactionbuilder.ADDRESS_LENGTH]byte{}
	copy(from_addr[:], mustDecodeHex(string(from)))
	copy(to_addr[:], mustDecodeHex(string(to)))
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
	// TODO validate max fee

	return &Tx{
		tx: transactionbuilder.RawTransaction{
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
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txb *TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input *tx_input.TxInput) (xc.Tx, error) {

	to_addr := [transactionbuilder.ADDRESS_LENGTH]byte{}
	from_addr := [transactionbuilder.ADDRESS_LENGTH]byte{}
	copy(from_addr[:], mustDecodeHex(string(from)))
	copy(to_addr[:], mustDecodeHex(string(to)))
	toAmountBytes := transactionbuilder.BCSSerializeBasicValue(amount.Int().Uint64())

	contract := txb.Asset.GetContract()

	typeTag, err := transactionbuilder.NewTypeTagStructFromString(contract)
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
	// TODO validate max fee
	return &Tx{
		tx: transactionbuilder.RawTransaction{
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
	}, nil
}
