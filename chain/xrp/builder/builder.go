package builder

import (
	"encoding/hex"
	"fmt"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xrp/address/contract"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	xrptx "github.com/cordialsys/crosschain/chain/xrp/tx"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	"github.com/shopspring/decimal"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

type TxInput = xrptxinput.TxInput
type Tx = xrptx.Tx

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(asset *xc.ChainBaseConfig) (*TxBuilder, error) {
	return &TxBuilder{
		Asset: asset,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	destinationTag := int64(0)
	var err error
	if memo, ok := args.GetMemo(); ok {
		destinationTag, err = strconv.ParseInt(memo, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("XRP memo must be a valid integer, got %s", memo)
		}
	}

	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args, contract, destinationTag, input)
	} else {
		return txBuilder.NewNativeTransfer(args, destinationTag, input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, destinationTag int64, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	XRPAmount := xrptx.AmountBlockchain{
		XRPAmount: args.GetAmount().String(),
	}
	pubKey, ok := args.GetPublicKey()
	if !ok || len(pubKey) == 0 {
		return nil, fmt.Errorf("must set from public-key in transfer args: %s", args.GetFrom())
	}

	xrpTx := xrptx.XRPTransaction{
		Account:            args.GetFrom(),
		Amount:             XRPAmount,
		Destination:        args.GetTo(),
		DestinationTag:     destinationTag,
		Fee:                txInput.Fee.String(),
		Flags:              0,
		LastLedgerSequence: txInput.V2LastLedgerSequence,
		Sequence:           txInput.V2Sequence,
		SigningPubKey:      hex.EncodeToString(pubKey),
		TransactionType:    xrptx.PAYMENT,
	}

	if txInput.AccountDelete {
		if args.InclusiveFeeSpendingEnabled() {
			// Only use account-delete if inclusive fee spending is enabled,
			// as only then is it ok to send an amount that could vary slightly.
			xrpTx.TransactionType = xrptx.ACCOUNT_DELETE
			xrpTx.Fee = txInput.AccountDeleteFee.String()
		}
	}

	return &xrptx.Tx{
		XRPTx:      &xrpTx,
		SignPubKey: pubKey,
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, assetId xc.ContractAddress, destinationTag int64, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	tokenAsset, tokenContract, err := contract.ExtractAssetAndContract(assetId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}
	pubKey, ok := args.GetPublicKey()
	if !ok || len(pubKey) == 0 {
		return nil, fmt.Errorf("must set from public-key in transfer args: %s", args.GetFrom())
	}

	// XRP tokens are fixed decimals
	bal := args.GetAmount()
	tokenAmountValue := bal.ToHuman(types.TRUSTLINE_DECIMALS)

	XRPAmount := xrptx.AmountBlockchain{
		TokenAmount: &xrptx.Amount{
			Currency: tokenAsset,
			Issuer:   tokenContract,
			Value:    tokenAmountValue.String(),
		},
	}

	// We permit spending an additional amount (10%) in order to send the target amount.
	// This is needed because XRP tokens can have their own fees.
	// https://xrpl.org/docs/concepts/payment-types/partial-payments#without-partial-payments
	sendMaxFactor, err := decimal.NewFromString("1.1")
	if err != nil {
		return nil, fmt.Errorf("error converting sendMaxFactor to decimal: %v", err)
	}

	sendMax := xrptx.Amount{
		Currency: tokenAsset,
		Issuer:   tokenContract,
		Value:    sendMaxFactor.Mul(tokenAmountValue.Decimal()).String(),
	}

	xrpTx := xrptx.XRPTransaction{
		Account:            args.GetFrom(),
		Amount:             XRPAmount,
		SendMax:            sendMax,
		Destination:        args.GetTo(),
		Fee:                txInput.Fee.String(),
		Flags:              0,
		LastLedgerSequence: txInput.V2LastLedgerSequence,
		Sequence:           txInput.V2Sequence,
		SigningPubKey:      hex.EncodeToString(pubKey),
		TransactionType:    xrptx.PAYMENT,
		DestinationTag:     destinationTag,
	}

	return &xrptx.Tx{
		XRPTx:      &xrpTx,
		SignPubKey: pubKey,
	}, nil
}
