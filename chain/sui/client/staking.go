package client

import (
	"context"
	"errors"
	"fmt"
	"slices"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/sui"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/go-sui-sdk/v2/move_types"
	"github.com/cordialsys/go-sui-sdk/v2/sui_types"
)

// Only stake discovery uses JSON-RPC; input preparation and simulation use gRPC.
func (c *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	suiAddress, err := move_types.NewAccountAddressHex(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("could not decode address: %w", err)
	}

	stakes, err := c.SuiClient.GetStakes(ctx, *suiAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get stakes: %w", err)
	}

	validator, _ := args.GetValidator()
	account, _ := args.GetAccount()
	return sui.SuiStakesToStakedBalances(stakes, validator, account), nil
}

// Sui minimum stake amount is 1.0 SUI
func (c *Client) isValidStakeAmount(amount xc.AmountBlockchain) bool {
	decimals := c.Asset.GetDecimals()
	minAmount := xc.NewAmountHumanReadableFromFloat(1.0).ToBlockchain(decimals)
	return amount.Cmp(&minAmount) != -1
}

func (c *Client) FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error) {
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	if !c.isValidStakeAmount(amount) {
		return nil, errors.New("minimal stake amount is 1.0 sui")
	}

	feePayer, _ := args.GetFeePayer()
	txInput, err := c.fetchBaseInput(ctx, sui.NativeCoin, args.GetFrom(), feePayer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch base input: %w", err)
	}

	stakingInput := &sui.StakingInput{
		*txInput,
	}
	builder, err := sui.NewTxBuilder(c.Asset.GetChain().Base())
	if err != nil {
		return nil, fmt.Errorf("failed to create tx builder: %w", err)
	}

	tx, err := builder.Stake(args, stakingInput)
	if err != nil {
		return nil, fmt.Errorf("could not build tx: %v", err)
	}
	// staking is always native
	isNative := true
	gasFee, ok, err := c.simulateTransactionGasFee(ctx, tx, isNative)
	if err != nil {
		return nil, fmt.Errorf("failed to get staking transaction gas fee: %w", err)
	}
	if !ok && stakingInput.AddressBalance != nil {
		return nil, fmt.Errorf("Sui simulation failed for address-balance transaction")
	}
	if ok {
		stakingInput.GasBudget = gasFee
		if stakingInput.AddressBalance != nil {
			_, err = builder.Stake(args, stakingInput)
			if err != nil {
				return nil, fmt.Errorf("insufficient funds for estimated gas: %w", err)
			}
			if stakingInput.GasBudget < gasFee {
				return nil, fmt.Errorf("insufficient SUI in gas funding source: need %d, have %d", gasFee, stakingInput.GasBudget)
			}
		}
	}

	return stakingInput, nil
}

// Fetch inputs required for a unstaking transaction
func (c *Client) FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error) {
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	decimals := c.Asset.GetDecimals()
	minAmount := xc.NewAmountHumanReadableFromFloat(1.0).ToBlockchain(decimals)
	if amount.Cmp(&minAmount) == -1 {
		return nil, fmt.Errorf("minimal unstake amount is %s SUI", minAmount.ToHuman(decimals))
	}
	feePayer, _ := args.GetFeePayer()
	txInput, err := c.fetchBaseInput(ctx, sui.NativeCoin, args.GetFrom(), feePayer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch base input: %w", err)
	}

	suiAddress, err := move_types.NewAccountAddressHex(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to encode address: %w", err)
	}

	rawStakes, err := c.SuiClient.GetStakes(ctx, *suiAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get stakes: %w", err)
	}

	validator, _ := args.GetValidator()
	account, _ := args.GetStakeAccount()
	stakeBalances := sui.SuiStakesToUnstakeInputs(rawStakes, validator, account)

	stakesToClose := make([]sui.Stake, 0)
	stakeToSplit := sui.Stake{}
	splitRemainder := xc.NewAmountBlockchainFromUint64(0)
	// check if there is any stake which amount is equal to unstake amount
	// and use it as only input
	if ok, stakeObject := sui.CanUnstakeAnyObject(stakeBalances, amount); ok {
		stakeId, err := sui.HexToAddress(stakeObject.ObjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to decode stakeId: %w", err)
		}

		objectDetails, err := c.getObject(ctx, sui_types.SuiAddress(stakeId).String())
		if err != nil {
			return nil, fmt.Errorf("failed to get stake object details: %w", err)
		}
		if objectDetails == nil {
			return nil, errors.New("invalid get object response")
		}

		stakeObject.Version = objectDetails.GetVersion()
		stakeObject.Digest = objectDetails.GetDigest()

		stakesToClose = append(stakesToClose, *stakeObject)
	} else {
		// there is no single stake object that satisfies this case
		// we have to prepare a series of unstake/split/merge operations, depending on the amount

		// sort stakes in ascending order, we want to close as much staking accounts as possible
		slices.SortFunc(stakeBalances[:], func(lhs, rhs sui.Stake) int {
			lhsBalance := lhs.GetBalance()
			rhsBalance := rhs.GetBalance()
			return lhsBalance.Cmp(&rhsBalance)
		})

		// split stakes into full unstakes and merge + split
		zeroAmount := xc.NewAmountBlockchainFromUint64(0)
		amountToUnstake := amount
		for _, s := range stakeBalances {
			// amount in sui.UnstakingInput stakes are sufficient for unstake operation
			if amountToUnstake.Cmp(&zeroAmount) <= 0 {
				break
			}

			stakeId, err := sui.HexToAddress(s.ObjectId)
			if err != nil {
				return nil, fmt.Errorf("failed to decode stake object id: %w", err)
			}

			objectDetails, err := c.getObject(ctx, sui_types.SuiAddress(stakeId).String())
			if err != nil {
				return nil, fmt.Errorf("failed to get stake object details: %w", err)
			}
			if objectDetails == nil {
				return nil, errors.New("invalid get object response")
			}

			s.Version = objectDetails.GetVersion()
			s.Digest = objectDetails.GetDigest()

			stakeBalance := s.GetBalance()
			remainingStake := stakeBalance.Sub(&amountToUnstake)
			remainingToUnstake := remainingStake.Abs()

			// unstake amount greater than the stake amount
			// make sure that remainingToUnstake is at least 'minAmount' - we cannot unstake smaller values
			if remainingStake.Cmp(&zeroAmount) <= -1 && remainingToUnstake.Cmp(&minAmount) >= 0 {
				stakesToClose = append(stakesToClose, s)
				amountToUnstake = amountToUnstake.Sub(&stakeBalance)
				continue
			}

			remainingAfterSplit, ok := s.TrySplit(amountToUnstake, minAmount, decimals)
			if ok {
				amountToUnstake = amountToUnstake.Sub(&remainingStake)
				stakeToSplit = s
				splitRemainder = remainingAfterSplit
				break
			}
		}

		amount, ok := args.GetAmount()
		if !ok {
			return nil, buildererrors.ErrStakingAmountRequired
		}
		if amountToUnstake.Cmp(&zeroAmount) > 0 {
			return nil, fmt.Errorf("cannot cover unstake amount (total: %v) with current stake objects, uncovered part: %v", amount, amountToUnstake)
		}
	}

	if len(stakesToClose) == 0 && stakeToSplit.GetBalance().Uint64() == 0 {
		return nil, fmt.Errorf("failed to unstake, missing valid stake objects")
	}

	unstakingInput := &sui.UnstakingInput{
		TxInput:         *txInput,
		StakesToUnstake: stakesToClose,
		StakeToSplit:    stakeToSplit,
		SplitAmount:     splitRemainder,
	}

	builder, err := sui.NewTxBuilder(c.Asset.GetChain().Base())
	if err != nil {
		return nil, fmt.Errorf("failed to create tx builder: %w", err)
	}

	tx, err := builder.Unstake(args, unstakingInput)
	if err != nil {
		return nil, fmt.Errorf("could not build tx: %v", err)
	}

	// staking is always native
	isNative := true
	gasFee, ok, err := c.simulateTransactionGasFee(ctx, tx, isNative)
	if err != nil {
		return nil, fmt.Errorf("failed to get unstaking transaction gas fee: %w", err)
	}
	if !ok && unstakingInput.AddressBalance != nil {
		return nil, fmt.Errorf("Sui simulation failed for address-balance transaction")
	}
	if ok {
		unstakingInput.GasBudget = gasFee
		if unstakingInput.AddressBalance != nil {
			_, err = builder.Unstake(args, unstakingInput)
			if err != nil {
				return nil, fmt.Errorf("insufficient funds for estimated gas: %w", err)
			}
			if unstakingInput.GasBudget < gasFee {
				return nil, fmt.Errorf("insufficient SUI in gas funding source: need %d, have %d", gasFee, unstakingInput.GasBudget)
			}
		}
	}
	return unstakingInput, nil
}

func (c *Client) FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error) {
	return nil, nil
}
