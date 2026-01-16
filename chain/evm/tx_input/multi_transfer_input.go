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

func (input *MultiTransferInput) GetNonce() uint64 {
	return input.Nonce
}

func (input *MultiTransferInput) GetFromAddress() string {
	return string(input.FromAddress)
}

func (input *MultiTransferInput) GetFeePayerNonce() uint64 {
	return input.FeePayerNonce
}

func (input *MultiTransferInput) GetFeePayerAddress() string {
	return string(input.FeePayerAddress)
}
