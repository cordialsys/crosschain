package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type BatchDepositInput struct {
	xc.StakingInputEnvelope
	TxInput
	PublicKeys [][]byte `json:"public_keys"`
	Signatures [][]byte `json:"signatures"`
}

var _ xc.VariantTxInput = &BatchDepositInput{}
var _ xc.StakeTxInput = &BatchDepositInput{}

func NewBatchDepositInput() *BatchDepositInput {
	return &BatchDepositInput{
		StakingInputEnvelope: *xc.NewStakingInputEnvelope(xc.EvmBatchDeposit),
	}
}
func (stakingInput *BatchDepositInput) GetVariant() xc.TxVariant {
	return stakingInput.Variant
}

// Mark as valid for staking transactions
func (*BatchDepositInput) Staking() {}
