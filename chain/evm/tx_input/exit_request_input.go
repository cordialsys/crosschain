package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type ExitRequestInput struct {
	TxInput
	PublicKeys [][]byte `json:"public_keys"`
}

var _ xc.TxVariantInput = &ExitRequestInput{}
var _ xc.UnstakeTxInput = &ExitRequestInput{}

func NewExitRequestInput() *ExitRequestInput {
	return &ExitRequestInput{}
}

func (*ExitRequestInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverEVM, "exit-request")
}

// Mark as valid for un-staking transactions
func (*ExitRequestInput) Unstaking() {}

func (input *ExitRequestInput) GetNonce() uint64 {
	return input.Nonce
}

func (input *ExitRequestInput) GetFromAddress() string {
	return string(input.FromAddress)
}

func (input *ExitRequestInput) GetFeePayerNonce() uint64 {
	return input.FeePayerNonce
}

func (input *ExitRequestInput) GetFeePayerAddress() string {
	return string(input.FeePayerAddress)
}
