package builder

import (
	"fmt"

	"cosmossdk.io/math"
	disttypes "cosmossdk.io/x/distribution/types"
	stakingtypes "cosmossdk.io/x/staking/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cosmos/cosmos-sdk/types"
)

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	stakeInput, ok := input.(*tx_input.StakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, stakeInput)
	}
	validatorAddress, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("validator address required to stake")
	}

	from := args.GetFrom()
	denom := txBuilder.GetDenom("")
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}

	msg := &stakingtypes.MsgDelegate{
		DelegatorAddress: string(from),
		ValidatorAddress: validatorAddress,
		Amount:           types.NewCoin(denom, math.NewIntFromBigInt(amount.Int())),
	}

	fees := txBuilder.calculateFees(amount, "", &stakeInput.TxInput, false)

	return txBuilder.createTxWithMsg(&stakeInput.TxInput, msg, tx.NewTxArgsFromStakingArgs(args, &stakeInput.TxInput), fees)
}

func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	stakeInput, ok := input.(*tx_input.UnstakingInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, stakeInput)
	}
	validatorAddress, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("validator address required to unstake")
	}

	from := args.GetFrom()

	denom := txBuilder.GetDenom("")
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}

	msg := &stakingtypes.MsgUndelegate{
		DelegatorAddress: string(from),
		ValidatorAddress: validatorAddress,
		Amount:           types.NewCoin(denom, math.NewIntFromBigInt(amount.Int())),
	}

	fees := txBuilder.calculateFees(amount, "", &stakeInput.TxInput, false)

	return txBuilder.createTxWithMsg(&stakeInput.TxInput, msg, tx.NewTxArgsFromStakingArgs(args, &stakeInput.TxInput), fees)
}

func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	withdrawInput, ok := input.(*tx_input.WithdrawInput)
	if !ok {
		return nil, fmt.Errorf("invalid input %T, expected %T", input, withdrawInput)
	}
	validatorAddress, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("validator address required to unstake")
	}

	from := args.GetFrom()

	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}

	// Cosmos automatically withdraws all rewards and unbonded balances (any input amount is ignored)
	msg := &disttypes.MsgWithdrawDelegatorReward{
		DelegatorAddress: string(from),
		ValidatorAddress: validatorAddress,
	}

	fees := txBuilder.calculateFees(amount, "", &withdrawInput.TxInput, false)

	return txBuilder.createTxWithMsg(&withdrawInput.TxInput, msg, tx.NewTxArgsFromStakingArgs(args, &withdrawInput.TxInput), fees)
}
