package builder

import xc "github.com/cordialsys/crosschain"

type FullTransferBuilder interface {
	Transfer
}
type FullBuilder interface {
	FullTransferBuilder
	Staking
}

// Marker to indicate if fee payer is supported, until we support it everywhere
type BuilderSupportsFeePayer interface {
	SupportsFeePayer()
}

type Transfer interface {
	Transfer(args TransferArgs, input xc.TxInput) (xc.Tx, error)
}

type Staking interface {
	Stake(stakingArgs StakeArgs, input xc.StakeTxInput) (xc.Tx, error)
	Unstake(stakingArgs StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error)
	Withdraw(stakingArgs StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error)
}
