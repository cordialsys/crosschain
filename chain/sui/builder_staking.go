package sui

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
)

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	var ok bool
	stakeInput, ok := input.(*StakingInput)
	if !ok {
		return &Tx{}, errors.New("xc.StakeTxInput is not from a sui chain")
	}

	amount := args.GetAmount()
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
	systemState, err := HexToObjectID("0x0000000000000000000000000000000000000000000000000000000000000005")
	if err != nil {
		return nil, fmt.Errorf("failed to encode sui system state: %w", err)
	}
	cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
		Value: &bcs.ObjectArg__SharedObject{
			Id:                   systemState,
			InitialSharedVersion: 1,
			Mutable:              true,
		},
	})

	validator, err := HexToPure(string(stakeInput.Validator))
	if err != nil {
		return nil, fmt.Errorf("failed to encode validator: %w", err)
	}
	validatorArg := ArgumentInput(uint16(len(cmd_inputs)))
	cmd_inputs = append(cmd_inputs, validator)

	// stake move command
	packageId, err := HexToObjectID("0x0000000000000000000000000000000000000000000000000000000000000003")
	if err != nil {
		return nil, fmt.Errorf("failed to decode objectId: %w", err)
	}
	commands = append(commands, &bcs.Command__MoveCall{
		Value: bcs.ProgrammableMoveCall{
			Package:       packageId,
			Module:        "sui_system",
			Function:      "request_add_stake",
			TypeArguments: []bcs.TypeTag{},
			Arguments: []bcs.Argument{
				systemStateArg,
				cointSplitResult,
				validatorArg,
			},
		},
	})

	// update transaction with stake commands
	txBase.SetInputsAndCommands(cmd_inputs, commands)

	xcTx := &Tx{
		Tx:         txBase.base,
		public_key: fromPubkey,
	}
	return xcTx, nil
}

func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	unstakeInput, ok := input.(*UnstakingInput)
	if !ok {
		return &Tx{}, errors.New("xc.StakeTxInput is not from a sui chain")
	}

	amount := args.GetAmount()
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
	systemState, err := HexToObjectID("0x0000000000000000000000000000000000000000000000000000000000000005")
	if err != nil {
		return nil, fmt.Errorf("failed to encode sui system state: %w", err)
	}
	cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
		Value: &bcs.ObjectArg__SharedObject{
			Id:                   systemState,
			InitialSharedVersion: 1,
			Mutable:              true,
		},
	})
	packageId, err := HexToObjectID("0x0000000000000000000000000000000000000000000000000000000000000003")
	if err != nil {
		return nil, fmt.Errorf("failed to decode objectId: %w", err)
	}

	// prepare unstake commands
	for _, s := range unstakeInput.StakesToUnstake {
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
				Package:       packageId,
				Module:        "sui_system",
				Function:      "request_withdraw_stake",
				TypeArguments: []bcs.TypeTag{},
				Arguments: []bcs.Argument{
					systemStateInput,
					stakeIdInput,
				},
			},
		})
	}

	// check if we have to prepare a split operation
	if unstakeInput.StakeToSplit.GetBalance().Uint64() != 0 {
		if unstakeInput.SplitRemainder.IsZero() {
			return nil, fmt.Errorf("split object is provided, but split remainder is incorrect")
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
		pureToSplitOff := U64ToPure(unstakeInput.SplitRemainder.Uint64())
		pureToSplitOffArg := ArgumentInput(uint16(len(cmd_inputs)))
		cmd_inputs = append(cmd_inputs, pureToSplitOff)

		// prepare split command
		commands = append(commands, &bcs.Command__MoveCall{
			Value: bcs.ProgrammableMoveCall{
				Package:       packageId,
				Module:        "staking_pool",
				Function:      "split_staked_sui",
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
				Package:       packageId,
				Module:        "sui_system",
				Function:      "request_withdraw_stake",
				TypeArguments: []bcs.TypeTag{},
				Arguments: []bcs.Argument{
					systemStateInput,
					mainStakeIdInput,
				},
			},
		})
	}

	txBase.SetInputsAndCommands(cmd_inputs, commands)

	xcTx := &Tx{
		Tx:         txBase.base,
		public_key: fromPubkey,
	}
	return xcTx, nil
}

func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return nil, errors.New("sui doesn't require a separate withdraw call")
}
