package builder

import (
	"encoding/hex"
	"encoding/json"
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

	signingPubKeyHex := hex.EncodeToString(txInput.Pubkey)
	//txInput.XRPTx.SigningPubKey = signingPubKeyHex
	fmt.Println(txInput)

	jsonData, err := json.Marshal(txInput.XRPTx)
	if err != nil {
		return nil, errors.New("error marshalling struct")
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, errors.New("error unmarshalling struct")
	}

	if sequence, ok := result["Sequence"].(float64); ok {
		result["Sequence"] = int(sequence)
	}

	if flags, ok := result["Flags"].(float64); ok {
		result["Flags"] = int(flags)
	}

	if lastLedgerSequence, ok := result["LastLedgerSequence"].(float64); ok {
		result["LastLedgerSequence"] = int(lastLedgerSequence)
	}

	serializedForSigning, err := binarycodec.EncodeForSigning(result)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("serializedForSigning:", serializedForSigning)

	txInput.SerializeXRPTx = serializedForSigning

	return &tx.Tx{
		XRPTx:             &txInput.XRPTx,
		SerialisedForSign: &serializedForSigning,
		SignPubKey:        &signingPubKeyHex,
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return nil, errors.New("not implemented")
}
