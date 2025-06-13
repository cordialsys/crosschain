package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

type MultiTransferInput struct {
	TxInput
}

func init() {
	registry.RegisterTxVariantInput(&MultiTransferInput{})
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
