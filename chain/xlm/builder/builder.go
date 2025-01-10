package builder

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xlmtx "github.com/cordialsys/crosschain/chain/xlm/tx"
	xlminput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go/xdr"
)

type TxBuilder struct {
	Asset xc.ITask
}

var _ xc.TxBuilder = &TxBuilder{}
var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

type TxInput = xlminput.TxInput
type Tx = xlmtx.Tx

func NewTxBuilder(asset xc.ITask) (*TxBuilder, error) {
	return &TxBuilder{
		Asset: asset,
	}, nil
}

func (builder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return builder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

func (builder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	switch builder.Asset.(type) {
	case *xc.ChainConfig:
		return builder.NewNativeTransfer(from, to, amount, input)
	case *xc.TokenAssetConfig:
	default:
		return &xlmtx.Tx{}, errors.New("not implemented")
	}

	return &xlmtx.Tx{}, errors.New("not implemented")
}

func (builder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	var sourceAccount xdr.MuxedAccount
	fromStr := string(from)
	err := sourceAccount.SetAddress(fromStr)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `from` address: %w", err)
	}

	preconditions := xlmtx.Preconditions{
		TimeBounds: xlmtx.NewTimeout(txInput.TransactionActiveTime),
	}

	txe := xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{
			SourceAccount: sourceAccount,
			// We can skip fee * operation_count multiplication because the transfer is a single
			// `Payment` operation
			Fee:    xdr.Uint32(txInput.MaxFee),
			SeqNum: xdr.SequenceNumber(txInput.Sequence),
			Cond:   preconditions.BuildXDR(),
		},
	}

	if txInput.Memo != "" {
		var xdrMemo xdr.Memo
		xdrMemo, err = xdr.NewMemo(xdr.MemoTypeMemoText, txInput.Memo)
		txe.Tx.Memo = xdrMemo
	}

	var destinationMuxedAccount xdr.MuxedAccount
	err = destinationMuxedAccount.SetAddress(string(to))
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address: %w", err)
	}
	xdrAmount := xdr.Int64(amount.Int().Int64())
	xdrAsset := xdr.Asset{
		Type: xdr.AssetTypeAssetTypeNative,
	}

	xdrPayment := xdr.PaymentOp{
		Destination: destinationMuxedAccount,
		Amount:      xdrAmount,
		Asset:       xdrAsset,
	}

	xdrOperationBody, err := xdr.NewOperationBody(xdr.OperationTypePayment, xdrPayment)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("failed to create operation body: %w", err)
	}

	xdrOperation := xdr.Operation{
		SourceAccount: &sourceAccount,
		Body:          xdrOperationBody,
	}

	txe.Tx.Operations = []xdr.Operation{xdrOperation}

	xlmTx, err := xdr.NewTransactionEnvelope(xdr.EnvelopeTypeEnvelopeTypeTx, txe)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("failed to create transaction envelope: %w", err)
	}

	return &xlmtx.Tx{
		TxEnvelope:        &xlmTx,
		NetworkPassphrase: txInput.Passphrase,
	}, nil
}
