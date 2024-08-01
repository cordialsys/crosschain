package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/gagliardetto/solana-go"
)

type StakingInput struct {
	TxInput
	ValidatorVoteAccount solana.PublicKey `json:"validator_vote_account"`
	// The new staking account to create
	StakingKey solana.PrivateKey `json:"staking_key"`
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*StakingInput) Staking() {}

func (*StakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverSolana, "native")
}

type ExistingStake struct {
	ActivationEpoch      uint64              `json:"activation_epoch"`
	DeactivationEpoch    uint64              `json:"deactivation_epoch"`
	Amount               xc.AmountBlockchain `json:"amount"`
	ValidatorVoteAccount string              `json:"validator_vote_account"`
	StakeAccount         string              `json:"stake_account"`
}
type UnstakingInput struct {
	TxInput

	// TODO do we need this?
	// The new staking account to create in the event of a split occuring
	StakingKey solana.PrivateKey `json:"staking_key"`

	CurrentEpoch   uint64           `json:"current_epoch"`
	ExistingStakes []*ExistingStake `json:"existing_stakes"`
}

var _ xc.TxVariantInput = &UnstakingInput{}
var _ xc.UnstakeTxInput = &UnstakingInput{}

func (*UnstakingInput) Unstaking() {}

func (*UnstakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverSolana, "native")
}
