package builder

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
)

// Stake routes to the appropriate sub-builder based on chain
func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	if txBuilder.Asset.Chain == xc.TAO {
		return NewTaoStakingBuilder(&txBuilder).Stake(args, input)
	}

	// For generic substrate chains using nomination pools
	return NewNominationPoolsStakingBuilder(&txBuilder).Stake(args, input)
}

// Unstake routes to the appropriate sub-builder based on chain
func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	if txBuilder.Asset.Chain == xc.TAO {
		return NewTaoStakingBuilder(&txBuilder).Unstake(args, input)
	}

	// For generic substrate chains using nomination pools
	return NewNominationPoolsStakingBuilder(&txBuilder).Unstake(args, input)
}

// Withdraw routes to the appropriate sub-builder based on chain
func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	if txBuilder.Asset.Chain == xc.TAO {
		return NewTaoStakingBuilder(&txBuilder).Withdraw(args, input)
	}

	// For generic substrate chains using nomination pools
	return NewNominationPoolsStakingBuilder(&txBuilder).Withdraw(args, input)
}
