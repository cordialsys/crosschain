package solana

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/gagliardetto/solana-go"
)

type StakingInput struct {
	TxInput
	// The validator vote account to stake with
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
