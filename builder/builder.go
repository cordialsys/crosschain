package builder

import xc "github.com/cordialsys/crosschain"

type FullTransferBuilder interface {
	Transfer
}
type FullBuilder interface {
	FullTransferBuilder
	Staking
}

type FeePayerType string

const (
	// For when the fee-payer has it's own nonce/sequence/utxo and can lead to conflicts
	FeePayerWithConflicts FeePayerType = "has-conflicts"
	// The fee-payer does not have any cause for conflicts (relies solely on main signer(s))
	FeePayerNoConflicts FeePayerType = "no-conflicts"
)

// Marker to indicate if fee payer is supported, until we support it everywhere
type BuilderSupportsFeePayer interface {
	SupportsFeePayer() FeePayerType
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
