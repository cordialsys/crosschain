package builder

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/xlm"
	"github.com/stellar/go/xdr"
	common "github.com/cordialsys/crosschain/chain/xlm/common"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xlminput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	xlmtx "github.com/cordialsys/crosschain/chain/xlm/tx"
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

// Implements xcbuilder/Transfer interface
func (builder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return builder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

// Implements xc.TxBuilder interface
func (builder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	sourceAccount, err := common.MuxedAccountFromAddress(from)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `from` address: %w", err)
	}

	preconditions := xlm.Preconditions{
		TimeBounds: xlm.NewTimeout(txInput.TransactionActiveTime),
	}

	txe := xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{
			SourceAccount: sourceAccount,
			// We can skip fee * operation_count multiplication because the transfer is a single
			// `Payment` operation
			Fee:    xdr.Uint32(50000000),
			SeqNum: xdr.SequenceNumber(txInput.Sequence),
			Cond:   preconditions.BuildXDR(),
		},
	}

	if txInput.Memo != "" {
		var xdrMemo xdr.Memo
		xdrMemo, err = xdr.NewMemo(xdr.MemoTypeMemoText, txInput.Memo)
		txe.Tx.Memo = xdrMemo
	}

	destinationMuxedAccount, err := common.MuxedAccountFromAddress(to)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address: %w", err)
	}
	xdrAmount := xdr.Int64(amount.Int().Int64())

	var xdrAsset xdr.Asset
	if tokenConfig, ok := builder.Asset.(*xc.TokenAssetConfig); ok {
		contractDetails, err := common.GetAssetAndIssuerFromContract(tokenConfig.Contract)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to get contract details: %w", err)
		}

		xdrAsset, err = common.CreateAssetFromContractDetails(contractDetails)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create token details: %w", err)
		}
	} else {
		xdrAsset.Type = xdr.AssetTypeAssetTypeNative
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
