package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type MultiTransferInput struct {
	TxInput
}

var _ xc.TxVariantInput = &MultiTransferInput{}
var _ xc.MultiTransferInput = &MultiTransferInput{}

func NewMultiTransferInput() *MultiTransferInput {
	return &MultiTransferInput{}
}

func (input *MultiTransferInput) GetVariant() xc.TxVariantInputType {
	return xc.NewMultiTransferInputType(xc.DriverEVM, "eip7702")
}

func (input *MultiTransferInput) MultiTransfer() {}
