package sui

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
)

const (
	moduleSuiSystem            = "sui_system"
	moduleStakingPool          = "staking_pool"
	methodRequestWithdrawStake = "request_withdraw_stake"
	methodSplitStakedSui       = "split_staked_sui"
	methodRequestAddStake      = "request_add_stake"
)

// both systemState and stakingPackageId are fixed
var systemState bcs.ObjectID = MustHexToObjectID("0x0000000000000000000000000000000000000000000000000000000000000005")
var stakingPackageId bcs.ObjectID = MustHexToObjectID("0x0000000000000000000000000000000000000000000000000000000000000003")

func ValidateStakeObject(stakeObject Stake, validator string, account string) error {
	if validator != "" && stakeObject.Validator != validator {
		return fmt.Errorf("input validator and args validator differ")
	}
	if account != "" && stakeObject.ObjectId != account {
		return fmt.Errorf("input account and args stake account differ")
	}

	return nil
}

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	var ok bool
	stakeInput, ok := input.(*StakingInput)
	if !ok {
		return &Tx{}, errors.New("xc.StakeTxInput is not from a sui chain")
	}

	validator, ok := args.GetValidator()
	if !ok {
		return &Tx{}, errors.New("empty validator")
	}

	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	feePayer, ok := args.GetFeePayer()
	if !ok {
		feePayer = args.GetFrom()
	}
	fromPubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, errors.New("sui transactions require pubkey")
	}

	txBase, err := txBuilder.newTransactionBase(
		feePayer,
		args.GetFrom(),
		amount,
		&stakeInput.TxInput,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction base: %w", err)
	}

	primaryCoinInput, commands, cmd_inputs, err := txBuilder.prepareGasSplitAndMergeCommands(feePayer,
		args.GetFrom(),
		stakeInput.TxInput,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare split and merge commands: %w", err)
	}

	// save split result for later use in MoveCall command
	cointSplitResult := ArgumentResult(uint16(len(commands)))
	// now let's spend the primary coin by splitting `amt` from it
	commands = append(commands, &bcs.Command__SplitCoins{
		Field0: primaryCoinInput,
		Field1: []bcs.Argument{
			// the last input has the amount
			ArgumentInput(uint16(len(cmd_inputs))),
		},
	})
	cmd_inputs = append(cmd_inputs, U64ToPure(amount.Uint64()))

	// add system state input
	systemStateArg := ArgumentInput(uint16(len(cmd_inputs)))
	cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
		Value: &bcs.ObjectArg__SharedObject{
			Id:                   systemState,
			InitialSharedVersion: 1,
			Mutable:              true,
		},
	})

	pureValidator, err := HexToPure(validator)
	if err != nil {
		return nil, fmt.Errorf("failed to encode validator: %w", err)
	}
	validatorArg := ArgumentInput(uint16(len(cmd_inputs)))
	cmd_inputs = append(cmd_inputs, pureValidator)

	commands = append(commands, &bcs.Command__MoveCall{
		Value: bcs.ProgrammableMoveCall{
			Package:       stakingPackageId,
			Module:        moduleSuiSystem,
			Function:      methodRequestAddStake,
			TypeArguments: []bcs.TypeTag{},
			Arguments: []bcs.Argument{
				systemStateArg,
				cointSplitResult,
				validatorArg,
			},
		},
	})

	xcTx := &Tx{
		Tx:         txBase.Build(cmd_inputs, commands),
		public_key: fromPubkey,
	}
	return xcTx, nil
}

func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	unstakeInput, ok := input.(*UnstakingInput)
	if !ok {
		return &Tx{}, errors.New("xc.StakeTxInput is not from a sui chain")
	}

	validator, _ := args.GetValidator()
	account, _ := args.GetStakeAccount()

	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	feePayer, ok := args.GetFeePayer()
	if !ok {
		feePayer = args.GetFrom()
	}
	fromPubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, errors.New("sui transactions require pubkey")
	}

	txBase, err := txBuilder.newTransactionBase(
		feePayer,
		args.GetFrom(),
		amount,
		&unstakeInput.TxInput,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction base: %w", err)
	}

	commands := make([]bcs.Command, 0)
	cmd_inputs := make([]bcs.CallArg, 0)

	// prepare system state input - it will be reused 'request_withdraw_stake' commands
	systemStateInput := ArgumentInput(uint16(len(cmd_inputs)))
	cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
		Value: &bcs.ObjectArg__SharedObject{
			Id:                   systemState,
			InitialSharedVersion: 1,
			Mutable:              true,
		},
	})

	// prepare unstake commands
	unstakedAmount := xc.NewAmountBlockchainFromUint64(0)
	for _, s := range unstakeInput.StakesToUnstake {
		err := ValidateStakeObject(unstakeInput.StakeToSplit, validator, account)
		if err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		stakeIdInput := ArgumentInput(uint16(len(cmd_inputs)))
		stakeId, err := HexToObjectID(s.ObjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to encode stake id object: %w", err)
		}
		stakeDigest, err := Base58ToObjectDigest(s.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to encode unstake digest: %w", err)
		}
		cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
			Value: &bcs.ObjectArg__ImmOrOwnedObject{
				Field0: stakeId,
				Field1: bcs.SequenceNumber(s.Version),
				Field2: stakeDigest,
			},
		})

		commands = append(commands, &bcs.Command__MoveCall{
			Value: bcs.ProgrammableMoveCall{
				Package:       stakingPackageId,
				Module:        moduleSuiSystem,
				Function:      methodRequestWithdrawStake,
				TypeArguments: []bcs.TypeTag{},
				Arguments: []bcs.Argument{
					systemStateInput,
					stakeIdInput,
				},
			},
		})
		stakeBalance := s.GetBalance()
		unstakedAmount = unstakedAmount.Add(&stakeBalance)
	}

	// check if we have to prepare a split operation
	if unstakeInput.StakeToSplit.GetBalance().Uint64() != 0 {
		err := ValidateStakeObject(unstakeInput.StakeToSplit, validator, account)
		if err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		if unstakeInput.SplitAmount.IsZero() {
			return nil, fmt.Errorf("split object is provided, but split remainder is incorrect")
		}
		balance := unstakeInput.StakeToSplit.GetBalance()
		remainder := balance.Sub(&unstakeInput.SplitAmount)
		unstakedAmount = unstakedAmount.Add(&remainder)
		if unstakedAmount.Cmp(&amount) != 0 {
			return nil, fmt.Errorf("input unstake(%v) amount and arg amount(%v) are different", unstakedAmount, amount)
		}
		mainStakeIdInput := ArgumentInput(uint16(len(cmd_inputs)))
		mainStakeId, err := HexToObjectID(unstakeInput.StakeToSplit.ObjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to encode merge id: %w", err)
		}
		mainStakeDigest, err := Base58ToObjectDigest(unstakeInput.StakeToSplit.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to encode main digest: %w", err)
		}
		cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
			Value: &bcs.ObjectArg__ImmOrOwnedObject{
				Field0: mainStakeId,
				Field1: bcs.SequenceNumber(unstakeInput.StakeToSplit.Version),
				Field2: mainStakeDigest,
			},
		})

		// leave only 'unstakedInput.SplitRemainder' balance on splited stake, so we can withdraw it with
		// 'request_withdraw_stake'
		pureToSplitOff := U64ToPure(unstakeInput.SplitAmount.Uint64())
		pureToSplitOffArg := ArgumentInput(uint16(len(cmd_inputs)))
		cmd_inputs = append(cmd_inputs, pureToSplitOff)

		// prepare split command
		commands = append(commands, &bcs.Command__MoveCall{
			Value: bcs.ProgrammableMoveCall{
				Package:       stakingPackageId,
				Module:        moduleStakingPool,
				Function:      methodSplitStakedSui,
				TypeArguments: []bcs.TypeTag{},
				Arguments: []bcs.Argument{
					mainStakeIdInput,
					pureToSplitOffArg,
				},
			},
		})

		// prepare unstake command, targeting value that remains on 'StakeToSplit' object
		commands = append(commands, &bcs.Command__MoveCall{
			Value: bcs.ProgrammableMoveCall{
				Package:       stakingPackageId,
				Module:        moduleSuiSystem,
				Function:      methodRequestWithdrawStake,
				TypeArguments: []bcs.TypeTag{},
				Arguments: []bcs.Argument{
					systemStateInput,
					mainStakeIdInput,
				},
			},
		})
	}

	xcTx := &Tx{
		Tx:         txBase.Build(cmd_inputs, commands),
		public_key: fromPubkey,
	}
	return xcTx, nil
}

func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return nil, errors.New("sui doesn't require a separate withdraw call")
}
