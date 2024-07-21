package builder

import xc "github.com/cordialsys/crosschain"

type FullBuilder interface {
	Transfer
	Staking
	xc.TxBuilder
}

type Transfer interface {
	Transfer(args TransferArgs, input xc.TxInput) (xc.Tx, error)
}

type Staking interface {
	Stake(stakingArgs StakeArgs, input xc.StakingInput) (xc.Tx, error)
	Unstake(stakingArgs StakeArgs, input xc.StakingInput) (xc.Tx, error)
}
