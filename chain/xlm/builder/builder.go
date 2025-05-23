package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xlm"
	common "github.com/cordialsys/crosschain/chain/xlm/common"
	xlmtx "github.com/cordialsys/crosschain/chain/xlm/tx"
	xlminput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go/xdr"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

type TxInput = xlminput.TxInput
type Tx = xlmtx.Tx

func NewTxBuilder(asset *xc.ChainBaseConfig) (*TxBuilder, error) {
	return &TxBuilder{
		Asset: asset,
	}, nil
}

func (builder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	from := args.GetFrom()
	to := args.GetTo()
	amount := args.GetAmount()

	txInput := input.(*TxInput)
	sourceAccount, err := common.MuxedAccountFromAddress(from)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `from` address: %w", err)
	}

	preconditions := xlm.Preconditions{
		TimeBounds: xlm.NewTimeout(txInput.TransactionActiveTime),
	}

	xdrMemo := xdr.Memo{}
	if memo, ok := args.GetMemo(); ok {
		xdrMemo, err = xdr.NewMemo(xdr.MemoTypeMemoText, memo)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create memo: %w", err)
		}
	}

	txe := xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{
			SourceAccount: sourceAccount,
			// We can skip fee * operation_count multiplication because the transfer is a single
			// `Payment` operation
			Fee:    xdr.Uint32(txInput.MaxFee),
			SeqNum: xdr.SequenceNumber(txInput.Sequence),
			Cond:   preconditions.BuildXDR(),
			Memo:   xdrMemo,
		},
	}

	if memo, ok := args.GetMemo(); ok {
		var xdrMemo xdr.Memo
		xdrMemo, err = xdr.NewMemo(xdr.MemoTypeMemoText, memo)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create memo: %w", err)
		}
		txe.Tx.Memo = xdrMemo
	}

	destinationMuxedAccount, err := common.MuxedAccountFromAddress(to)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address: %w", err)
	}
	xdrAmount := xdr.Int64(amount.Int().Int64())

	var xdrAsset xdr.Asset
	if contract, ok := args.GetContract(); ok {
		contractDetails, err := common.GetAssetAndIssuerFromContract(string(contract))
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
