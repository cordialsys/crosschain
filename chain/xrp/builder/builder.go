package builder

import (
	"encoding/hex"
	"fmt"
	xc "github.com/cordialsys/crosschain"
	xrptx "github.com/cordialsys/crosschain/chain/xrp/tx"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	"github.com/sirupsen/logrus"
	binarycodec "github.com/xyield/xrpl-go/binary-codec"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

var _ xc.TxBuilder = &TxBuilder{}

type TxInput = xrptxinput.TxInput
type Tx = xrptx.Tx

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
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
		StringValue: amount.String(),
		IsString:    true,
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

	resultMapXRP := xrptx.RenderToMap(xrpTx)
	resultMapWithAmount := xrptx.WithTokenAmount(resultMapXRP, XRPAmount.StringValue)

	encodeForSigningHex, err := binarycodec.EncodeForSigning(resultMapWithAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction for signing %v", err)
	}

	encodeForSigningBytes, err := hex.DecodeString(encodeForSigningHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte object from hex serialized transaction %v", err)
	}

	return &xrptx.Tx{
		XRPTx:            &xrpTx,
		SignPubKey:       txInput.PublicKey,
		EncodeForSigning: encodeForSigningBytes,
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

	tokenAsset, tokenContract, err := xrptx.ExtractAssetAndContract(assetContract)
	if err != nil {
		return nil, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}

	XRPAmount := xrptx.AmountBlockchain{
		AmountValue: &xrptx.Amount{
			Currency: tokenAsset,
			Issuer:   tokenContract,
			Value:    amount.String(),
		},
	}

	//amountResult := make(map[string]interface{})
	//amountResult["currency"] = tokenAsset
	//amountResult["issuer"] = tokenContract
	//amountResult["value"] = amount.String()

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

	resultMapXRP := xrptx.RenderToMap(xrpTx)
	resultMapWithAmount := xrptx.WithTokenAmount(resultMapXRP, XRPAmount.AmountValue)

	//result := make(map[string]interface{})
	//result["Account"] = string(xrpTx.Account)
	//result["Amount"] = amountResult
	//result["Destination"] = string(xrpTx.Destination)
	//result["Fee"] = xrpTx.Fee
	//result["Flags"] = int(xrpTx.Flags)
	//result["LastLedgerSequence"] = int(xrpTx.LastLedgerSequence)
	//result["Sequence"] = int(xrpTx.Sequence)
	//result["SigningPubKey"] = xrpTx.SigningPubKey
	//result["TransactionType"] = string(xrpTx.TransactionType)

	encodeForSigningHex, err := binarycodec.EncodeForSigning(resultMapWithAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction for signing %v", err)
	}

	encodeForSigningBytes, err := hex.DecodeString(encodeForSigningHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte object from hex serialized transaction %v", err)
	}

	return &xrptx.Tx{
		XRPTx:            &xrpTx,
		SignPubKey:       txInput.PublicKey,
		EncodeForSigning: encodeForSigningBytes,
	}, nil
}
