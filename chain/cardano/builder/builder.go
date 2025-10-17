package builder

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	tx "github.com/cordialsys/crosschain/chain/cardano/tx"
	cardanoinput "github.com/cordialsys/crosschain/chain/cardano/tx_input"
)

// Cardano TxBuilder
type TxBuilder struct{}

type TxInput = cardanoinput.TxInput

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xcbuilder.Staking = &TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return tx.NewTransfer(args, input)
}

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	return tx.NewStake(args, input)
}

func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	return tx.NewUnstake(args, input)
}

func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return tx.NewWithdraw(args, input)
}
