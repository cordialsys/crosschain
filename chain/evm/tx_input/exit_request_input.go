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

func (inp *ExitRequestInput) GetBaseTxInput() xc.TxInput { return &inp.TxInput }

func (*ExitRequestInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverEVM, "exit-request")
}

// Mark as valid for un-staking transactions
func (*ExitRequestInput) Unstaking() {}
