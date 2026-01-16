package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

func init() {
	registry.RegisterTxVariantInput(&CallInput{})
}

type CallInput struct {
	// base tx input
	TxInput
	// no additional info is needed for evm call currently
}

var _ xc.TxVariantInput = &CallInput{}
var _ xc.CallTxInput = &CallInput{}

func NewCallInput() *CallInput {
	return &CallInput{}
}

func (*CallInput) GetVariant() xc.TxVariantInputType {
	return xc.NewCallingInputType(xc.DriverEVM)
}

// Mark as valid for calling transactions
func (*CallInput) Calling() {}

func (input *CallInput) GetNonce() uint64 {
	return input.Nonce
}

func (input *CallInput) GetFromAddress() string {
	return string(input.FromAddress)
}

func (input *CallInput) GetFeePayerNonce() uint64 {
	return input.FeePayerNonce
}

func (input *CallInput) GetFeePayerAddress() string {
	return string(input.FeePayerAddress)
}
