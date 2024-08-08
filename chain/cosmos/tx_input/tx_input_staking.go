package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

type StakingInput struct {
	TxInput
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*StakingInput) Staking() {}

func (*StakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverCosmos, string(xc.Native))
}

type UnstakingInput struct {
	TxInput
}

var _ xc.TxVariantInput = &UnstakingInput{}
var _ xc.UnstakeTxInput = &UnstakingInput{}

func (*UnstakingInput) Unstaking() {}

func (*UnstakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverCosmos, string(xc.Native))
}

type WithdrawInput struct {
	TxInput
}

var _ xc.TxVariantInput = &WithdrawInput{}
var _ xc.WithdrawTxInput = &WithdrawInput{}

func (*WithdrawInput) GetVariant() xc.TxVariantInputType {
	return xc.NewWithdrawingInputType(xc.DriverCosmos, string(xc.Native))
}
func (*WithdrawInput) Withdrawing() {}
