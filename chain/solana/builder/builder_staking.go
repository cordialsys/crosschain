package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/gagliardetto/solana-go"
	compute_budget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/stake"
	"github.com/gagliardetto/solana-go/programs/system"
)

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	stakeInput, ok := input.(*tx_input.StakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, stakeInput)
	}
	_, ok = args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("validator to be delegated to is required")
	}

	// the sender/signer is the staking authority & withdraw authority
	stakingAuth, err := solana.PublicKeyFromBase58(string(args.GetFrom()))
	if err != nil {
		return nil, err
	}
	stakeAccountPub := stakeInput.StakingKey.PublicKey()
	instructions := []solana.Instruction{}
	instructions = append(instructions,
		// set gas fee priority
		compute_budget.NewSetComputeUnitPriceInstruction(
			stakeInput.GetLimitedPrioritizationFee(txBuilder.Asset.GetChain()),
		).Build(),
	)

	instructions = append(instructions,
		// create a new account for the stake
		system.NewCreateAccountInstruction(args.GetAmount().Uint64(), 200, solana.StakeProgramID, stakingAuth, stakeAccountPub).Build(),
	)
	instructions = append(instructions,
		// initialize the new account as staking account
		stake.NewInitializeInstruction(stakingAuth, stakingAuth, stakeAccountPub).Build(),
	)
	instructions = append(instructions,
		// delegate the stake to the validator
		stake.NewDelegateStakeInstruction(stakeInput.ValidatorVoteAccount, stakingAuth, stakeAccountPub).Build(),
	)
	tx, err := txBuilder.buildSolanaTx(instructions, stakingAuth, &stakeInput.TxInput)
	if err != nil {
		return nil, err
	}
	// The transient key behind the new stake account must sign the transaction also
	tx.AddTransientSigner(stakeInput.StakingKey)
	return tx, nil
}
func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("unimplemented")
}
