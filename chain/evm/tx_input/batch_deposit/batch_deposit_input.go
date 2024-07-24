package batch_deposit

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
)

type BatchDepositInput struct {
	xc.StakingInputEnvelope
	tx_input.TxInput
	PublicKeys [][]byte `json:"public_keys"`
	Signatures [][]byte `json:"signatures"`
}

var _ xc.VariantTxInput = &BatchDepositInput{}
var _ xc.StakeTxInput = &BatchDepositInput{}

func New() *BatchDepositInput {
	return &BatchDepositInput{
		StakingInputEnvelope: *xc.NewStakingInputEnvelope(xc.EvmBatchDeposit),
	}
}
func (stakingInput *BatchDepositInput) GetVariant() xc.TxVariant {
	return stakingInput.Variant
}

// Mark as valid for staking transactions
func (*BatchDepositInput) Staking() {}
