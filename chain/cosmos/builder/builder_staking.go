package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cosmos/cosmos-sdk/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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

	denom := txBuilder.GetDenom()
	amount := args.GetAmount()

	msg := &stakingtypes.MsgDelegate{
		DelegatorAddress: string(from),
		ValidatorAddress: validatorAddress,
		Amount:           types.NewCoin(denom, types.NewIntFromBigInt(amount.Int())),
	}

	fees := txBuilder.calculateFees(amount, &stakeInput.TxInput, false)
	memo, _ := args.GetMemo()
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("associated public key for %s was not passed as an argument", from)
	}

	return txBuilder.createTxWithMsg(&stakeInput.TxInput, msg, txArgs{
		Memo:          memo,
		FromPublicKey: pubkey,
	}, fees)
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

	denom := txBuilder.GetDenom()
	amount := args.GetAmount()

	msg := &stakingtypes.MsgUndelegate{
		DelegatorAddress: string(from),
		ValidatorAddress: validatorAddress,
		Amount:           types.NewCoin(denom, types.NewIntFromBigInt(amount.Int())),
	}

	fees := txBuilder.calculateFees(amount, &stakeInput.TxInput, false)
	memo, _ := args.GetMemo()
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("associated public key for %s was not passed as an argument", from)
	}

	return txBuilder.createTxWithMsg(&stakeInput.TxInput, msg, txArgs{
		Memo:          memo,
		FromPublicKey: pubkey,
	}, fees)
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

	amount := args.GetAmount()

	// Cosmos automatically withdraws all rewards and unbonded balances (any input amount is ignored)
	msg := &disttypes.MsgWithdrawDelegatorReward{
		DelegatorAddress: string(from),
		ValidatorAddress: validatorAddress,
	}

	fees := txBuilder.calculateFees(amount, &withdrawInput.TxInput, false)
	memo, _ := args.GetMemo()
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("associated public key for %s was not passed as an argument", from)
	}

	return txBuilder.createTxWithMsg(&withdrawInput.TxInput, msg, txArgs{
		Memo:          memo,
		FromPublicKey: pubkey,
	}, fees)
}
