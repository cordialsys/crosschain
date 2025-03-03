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

	from := args.GetFrom()
	to := args.GetTo()
	amount := args.GetAmount()

	destinationTag := int64(0)
	var err error
	if memo, ok := args.GetMemo(); ok {
		destinationTag, err = strconv.ParseInt(memo, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("XRP memo must be a valid integer, got %s", memo)
		}
	}

	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(from, to, amount, contract, destinationTag, input)
	} else {
		return txBuilder.NewNativeTransfer(from, to, amount, destinationTag, input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, destinationTag int64, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	XRPAmount := xrptx.AmountBlockchain{
		XRPAmount: amount.String(),
	}

	xrpTx := xrptx.XRPTransaction{
		Account:            from,
		Amount:             XRPAmount,
		Destination:        to,
		DestinationTag:     destinationTag,
		Fee:                txInput.Fee.String(),
		Flags:              0,
		LastLedgerSequence: txInput.LastLedgerSequence,
		Sequence:           txInput.Sequence,
		SigningPubKey:      hex.EncodeToString(txInput.PublicKey),
		TransactionType:    xrptx.PAYMENT,
	}

	return &xrptx.Tx{
		XRPTx:      &xrpTx,
		SignPubKey: txInput.PublicKey,
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, assetId xc.ContractAddress, destinationTag int64, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	tokenAsset, tokenContract, err := contract.ExtractAssetAndContract(assetId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}

	// XRP tokens are fixed decimals
	tokenAmountValue := amount.ToHuman(types.TRUSTLINE_DECIMALS)

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
		Account:            from,
		Amount:             XRPAmount,
		SendMax:            sendMax,
		Destination:        to,
		Fee:                txInput.Fee.String(),
		Flags:              0,
		LastLedgerSequence: txInput.LastLedgerSequence,
		Sequence:           txInput.Sequence,
		SigningPubKey:      hex.EncodeToString(txInput.PublicKey),
		TransactionType:    xrptx.PAYMENT,
		DestinationTag:     destinationTag,
	}

	return &xrptx.Tx{
		XRPTx:      &xrpTx,
		SignPubKey: txInput.PublicKey,
	}, nil
}
