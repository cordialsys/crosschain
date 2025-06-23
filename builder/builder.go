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

// Indicator to show that this chain requires an identity to be set for destination addresses.
// * The identity may be looked up dynamically for sending addresses
// * This is currently only used by EOS.
type BuilderRequiresIdentity interface {
	RequiresIdentityEOS()
}

type Transfer interface {
	Transfer(args TransferArgs, input xc.TxInput) (xc.Tx, error)
}

type MultiTransfer interface {
	MultiTransfer(args MultiTransferArgs, input xc.MultiTransferInput) (xc.Tx, error)
}

type Staking interface {
	Stake(stakingArgs StakeArgs, input xc.StakeTxInput) (xc.Tx, error)
	Unstake(stakingArgs StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error)
	Withdraw(stakingArgs StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error)
}
