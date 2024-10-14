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
	"github.com/sirupsen/logrus"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

var _ xc.TxBuilder = &TxBuilder{}
var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

type TxInput = xrptxinput.TxInput
type Tx = xrptx.Tx

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(asset xc.ITask) (*TxBuilder, error) {
	return &TxBuilder{
		Asset: asset,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	switch asset := txBuilder.Asset.(type) {
	case *xc.ChainConfig:
		return txBuilder.NewNativeTransfer(from, to, amount, input)
	case *xc.TokenAssetConfig:
		return txBuilder.NewTokenTransfer(from, to, amount, input)
	default:
		contract := asset.GetContract()
		logrus.WithFields(logrus.Fields{
			"chain":      asset.GetChain().Chain,
			"contract":   contract,
			"asset_type": fmt.Sprintf("%T", asset),
		}).Warn("new transfer for unknown asset type")
		if contract != "" {
			return txBuilder.NewTokenTransfer(from, to, amount, input)
		} else {
			return txBuilder.NewNativeTransfer(from, to, amount, input)
		}
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	XRPAmount := xrptx.AmountBlockchain{
		XRPAmount: amount.String(),
	}

	xrpTx := xrptx.XRPTransaction{
		Account:            from,
		Amount:             XRPAmount,
		Destination:        to,
		Fee:                "10",
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
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	asset := txBuilder.Asset
	txInput := input.(*TxInput)

	assetContract := asset.GetContract()
	if assetContract == "" {
		return nil, fmt.Errorf("asset does not have a contract")
	}

	tokenAsset, tokenContract, err := contract.ExtractAssetAndContract(assetContract)
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

	var destinationTag int64
	if txInput.LegacyMemo != "" {
		destinationTag, err = strconv.ParseInt(txInput.LegacyMemo, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting destinationTag to int64: %v", err)
		}
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
		Fee:                "10",
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
