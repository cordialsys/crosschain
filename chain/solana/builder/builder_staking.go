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

const RentExemptLamportsThreshold = 3000000
const RentExemptLamportsThresholdHuman = "0.003"
const StakeAccountSize = 200

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	stakeInput, ok := input.(*tx_input.StakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, stakeInput)
	}
	_, ok = args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("validator to be delegated to is required")
	}
	amount := args.GetAmount().Uint64()
	if amount < RentExemptLamportsThreshold {
		return nil, fmt.Errorf("amount to unstake is below the rent exempt threshold (%s SOL)", RentExemptLamportsThresholdHuman)
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
			stakeInput.GetPrioritizationFee(),
		).Build(),
	)

	instructions = append(instructions,
		// create a new account for the stake
		system.NewCreateAccountInstruction(args.GetAmount().Uint64(), StakeAccountSize, solana.StakeProgramID, stakingAuth, stakeAccountPub).Build(),
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
	unstakeInput, ok := input.(*tx_input.UnstakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, unstakeInput)
	}
	amount := args.GetAmount().Uint64()
	if amount < RentExemptLamportsThreshold {
		return nil, fmt.Errorf("amount to unstake is below the rent exempt threshold (%s SOL)", RentExemptLamportsThresholdHuman)
	}
	// the sender/signer is the staking authority & withdraw authority
	stakingAuth, err := solana.PublicKeyFromBase58(string(args.GetFrom()))
	if err != nil {
		return nil, err
	}

	accountsToConsume := []*tx_input.ExistingStake{}
	total := uint64(0)

	for _, stake := range unstakeInput.EligibleStakes {
		accountsToConsume = append(accountsToConsume, stake)
		// include the inactive amount in the total, as it's part of the principal,
		// and is also withdrawn.  Without including it, then a user won't see the
		// same amount withdrawn as they do when they unstake - it will seem like they are unstake a little more.
		total += (stake.AmountActive.Uint64() + stake.AmountInactive.Uint64())
		if total >= amount {
			break
		}
	}

	if total < amount-uint64(RentExemptLamportsThreshold*len(accountsToConsume)) {
		return nil, fmt.Errorf("insufficient amount staked to unstake")
	}
	if len(accountsToConsume) == 0 {
		return nil, fmt.Errorf("no stake accounts found to unstake")
	}
	if len(accountsToConsume) > MaxAccountUnstakes {
		return nil, fmt.Errorf("cannot unstake %d stake accounts to satisfy unstaking target amount, try unstaking a smaller amount", len(accountsToConsume))
	}

	remainder := uint64(0)
	if total > amount+RentExemptLamportsThreshold {
		remainder = total - amount
	}

	instructions := []solana.Instruction{}
	instructions = append(instructions,
		// set gas fee priority
		compute_budget.NewSetComputeUnitPriceInstruction(
			unstakeInput.GetPrioritizationFee(),
		).Build(),
	)
	didSplit := false

	if remainder > 0 {
		// need to split one of the stake account into a new first, to leave the remainder still staked
		var stakeAccountToSplit *tx_input.ExistingStake
		for _, stake := range accountsToConsume {
			// take first account that has enough to cover the remainder and can leave some for rent
			if stake.AmountActive.Uint64() > remainder+RentExemptLamportsThreshold*2 {
				stakeAccountToSplit = stake
				break
			}
		}
		if stakeAccountToSplit != nil {
			stakeAccountPub := unstakeInput.StakingKey.PublicKey()
			instructions = append(instructions,
				// create a new account for the split stake
				system.NewCreateAccountInstruction(0, StakeAccountSize, solana.StakeProgramID, stakingAuth, stakeAccountPub).Build(),
			)
			// split the stake account
			instructions = append(instructions,
				stake.NewSplitInstruction(
					remainder,
					stakeAccountToSplit.StakeAccount,
					stakeAccountPub,
					stakingAuth,
				).Build(),
			)
			didSplit = true
		}
	}

	// now unstake/consume all of the accounts
	for _, stakeAccount := range accountsToConsume {
		instructions = append(instructions,
			// deactive the stake account
			stake.NewDeactivateInstruction(stakeAccount.StakeAccount, stakingAuth).Build(),
		)
	}

	tx, err := txBuilder.buildSolanaTx(instructions, stakingAuth, &unstakeInput.TxInput)
	if err != nil {
		return nil, err
	}
	if didSplit {
		// The transient key behind the new stake account must sign the transaction also
		tx.AddTransientSigner(unstakeInput.StakingKey)
	}
	return tx, nil
}

func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	withdrawInput, ok := input.(*tx_input.WithdrawInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, withdrawInput)
	}
	amount := args.GetAmount().Uint64()
	// the sender/signer is the staking authority & withdraw authority
	stakingAuth, err := solana.PublicKeyFromBase58(string(args.GetFrom()))
	if err != nil {
		return nil, err
	}

	instructions := []solana.Instruction{}
	instructions = append(instructions,
		// set gas fee priority
		compute_budget.NewSetComputeUnitPriceInstruction(
			withdrawInput.GetPrioritizationFee(),
		).Build(),
	)

	total := uint64(0)
	for _, stakeAccount := range withdrawInput.EligibleStakes {
		amountToWithdraw := stakeAccount.AmountInactive.Uint64()
		total += amountToWithdraw
		if total > amount {
			// only withdraw the amount needed
			amountToWithdraw -= (total - amount)
		}
		n := stake.NewWithdrawInstruction(
			amountToWithdraw,
			stakeAccount.StakeAccount,
			stakingAuth,
			stakingAuth,
		)
		fmt.Println("amountToWithdraw", *n.Lamports)
		instructions = append(instructions,
			// withdraw from stake account
			n.Build(),
		)
		if total >= amount {
			break
		}
	}

	if total < amount {
		return nil, fmt.Errorf("insufficient amount in inactive stake(s) to withdraw")
	}
	if total == 0 {
		return nil, fmt.Errorf("no inactive stake accounts found to withdraw from")
	}
	if len(instructions) > MaxAccountWithdraws+1 {
		return nil, fmt.Errorf("cannot withdraw from %d stake accounts to satisfy unstaking target amount, try withdrawing a smaller amount", len(instructions)-1)
	}

	tx, err := txBuilder.buildSolanaTx(instructions, stakingAuth, &withdrawInput.TxInput)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
