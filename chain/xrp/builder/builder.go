package builder

import (
	"encoding/hex"
	"errors"
	"fmt"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	binarycodec "github.com/xyield/xrpl-go/binary-codec"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

var _ xc.TxBuilder = &TxBuilder{}

type TxInput = xrptxinput.TxInput

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	if _, ok := txBuilder.Asset.(*xc.TokenAssetConfig); ok {
		return txBuilder.NewTokenTransfer(from, to, amount, input)
	}
	return txBuilder.NewNativeTransfer(from, to, amount, input)
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	xrpTx := tx.XRPTransaction{
		Account:     from,
		Amount:      amount,
		Destination: to,
		Fee:         "12",
		Flags:       0,
		//LastLedgerSequence: txInput.LastLedgerSequence,
		Sequence:        txInput.Sequence,
		SigningPubKey:   hex.EncodeToString(txInput.PublicKey),
		TransactionType: tx.PAYMENT,
	}

	result := make(map[string]interface{})
	result["Account"] = string(xrpTx.Account)
	result["Amount"] = xrpTx.Amount.String()
	result["Destination"] = string(xrpTx.Destination)
	result["Fee"] = xrpTx.Fee
	result["Flags"] = xrpTx.Flags
	//result["LastLedgerSequence"] = xrpTx.LastLedgerSequence
	result["Sequence"] = xrpTx.Sequence
	result["SigningPubKey"] = xrpTx.SigningPubKey
	result["TransactionType"] = string(xrpTx.TransactionType)

	encodeForSigning, err := binarycodec.EncodeForSigning(result)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction for signing %v", err)
	}

	encodeForSigningBytes, err := hex.DecodeString(encodeForSigning)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte object from hex serialized transaction %v", err)
	}

	return &tx.Tx{
		XRPTx:            &xrpTx,
		SignPubKey:       txInput.PublicKey,
		EncodeForSigning: encodeForSigningBytes,
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return nil, errors.New("not implemented")
}
