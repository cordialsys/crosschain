package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type BatchDepositInput struct {
	TxInput
	PublicKeys [][]byte `json:"public_keys"`
	Signatures [][]byte `json:"signatures"`
}

var _ xc.TxVariantInput = &BatchDepositInput{}
var _ xc.StakeTxInput = &BatchDepositInput{}

func NewBatchDepositInput() *BatchDepositInput {
	return &BatchDepositInput{}
}

// Mark as valid for staking transactions
func (*BatchDepositInput) Staking() {}

func (*BatchDepositInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverEVM, "batch-deposit")
}
