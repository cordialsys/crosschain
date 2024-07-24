package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type ExitRequestInput struct {
	xc.StakingInputEnvelope
	TxInput
	PublicKeys [][]byte `json:"public_keys"`
}

var _ xc.VariantTxInput = &ExitRequestInput{}
var _ xc.UnstakeTxInput = &ExitRequestInput{}

func NewExitRequestInput() *ExitRequestInput {
	return &ExitRequestInput{
		StakingInputEnvelope: *xc.NewStakingInputEnvelope(xc.EvmRequestExitDeposit),
	}
}

func (stakingInput *ExitRequestInput) GetVariant() xc.TxVariant {
	return stakingInput.Variant
}

// Mark as valid for un-staking transactions
func (*ExitRequestInput) Unstaking() {}
