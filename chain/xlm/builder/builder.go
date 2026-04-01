package builder

import (
	"fmt"
	"math"

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
var _ xcbuilder.BuilderSupportsFeePayer = &TxBuilder{}

func (builder TxBuilder) SupportsFeePayer() xcbuilder.FeePayerType {
	return xcbuilder.FeePayerWithConflicts
}

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

	// Determine the transaction source account (who pays fees and provides sequence)
	txSourceAddress := from
	txSequence := txInput.GetXlmSequence()
	feePayer, hasFeePayer := args.GetFeePayer()
	if hasFeePayer {
		txSourceAddress = feePayer
		txSequence = int64(txInput.FeePayerSequence)
	}

	txSourceAccount, err := common.MuxedAccountFromAddress(txSourceAddress)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid transaction source address: %w", err)
	}

	// The operation source is always the sender
	opSourceAccount, err := common.MuxedAccountFromAddress(from)
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
			SourceAccount: txSourceAccount,
			// We can skip fee * operation_count multiplication because the transfer is a single
			// `Payment` operation
			Fee:    xdr.Uint32(txInput.MaxFee),
			SeqNum: xdr.SequenceNumber(txSequence),
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

	var xdrOperationBody xdr.OperationBody
	_, isToken := args.GetContract()

	if !txInput.DestinationFunded && !isToken {
		// Use CreateAccount for new/unfunded native XLM destinations
		destinationAccountId, err := xdr.AddressToAccountId(string(to))
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address: %w", err)
		}
		xdrCreateAccount := xdr.CreateAccountOp{
			Destination:     destinationAccountId,
			StartingBalance: xdrAmount,
		}
		xdrOperationBody, err = xdr.NewOperationBody(xdr.OperationTypeCreateAccount, xdrCreateAccount)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create operation body: %w", err)
		}
	} else {
		// Use Payment for funded accounts or token transfers
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
		xdrOperationBody, err = xdr.NewOperationBody(xdr.OperationTypePayment, xdrPayment)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create operation body: %w", err)
		}
	}

	var operations []xdr.Operation

	// If the sender needs a trustline for the token, prepend a ChangeTrust operation.
	if txInput.NeedsCreateTrustline {
		if contract, ok := args.GetContract(); ok {
			contractDetails, err := common.GetAssetAndIssuerFromContract(string(contract))
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to get contract details for trustline: %w", err)
			}
			changeTrustAsset, err := common.CreateChangeTrustAsset(contractDetails)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create change trust asset: %w", err)
			}
			changeTrustBody, err := xdr.NewOperationBody(xdr.OperationTypeChangeTrust, xdr.ChangeTrustOp{
				Line:  changeTrustAsset,
				Limit: xdr.Int64(math.MaxInt64),
			})
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create change trust operation: %w", err)
			}
			operations = append(operations, xdr.Operation{
				SourceAccount: &opSourceAccount,
				Body:          changeTrustBody,
			})
		}
	}

	// Skip the payment/createAccount operation when amount is 0 (trustline-only transaction)
	if xdrAmount > 0 {
		operations = append(operations, xdr.Operation{
			SourceAccount: &opSourceAccount,
			Body:          xdrOperationBody,
		})
	}

	txe.Tx.Operations = operations

	xlmTx, err := xdr.NewTransactionEnvelope(xdr.EnvelopeTypeEnvelopeTypeTx, txe)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("failed to create transaction envelope: %w", err)
	}

	tx := &xlmtx.Tx{
		TxEnvelope:        &xlmTx,
		NetworkPassphrase: txInput.Passphrase,
	}
	if hasFeePayer {
		tx.FeePayer = feePayer
	}
	return tx, nil
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	// XLM supports memo
	return xc.MemoSupportString
}
