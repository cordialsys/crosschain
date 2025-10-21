package sui

import (
	"context"
	"errors"
	"fmt"
	"slices"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/go-sui-sdk/v2/move_types"
	"github.com/cordialsys/go-sui-sdk/v2/sui_types"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/sirupsen/logrus"
)

var _ xclient.StakingClient = &Client{}

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
	return SuiStakesToStakedBalances(stakes, validator, account), nil
}

// Sui min stake amnount is 1.0 SUI
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
	txInput, err := c.fetchBaseInput(ctx, NativeCoin, args.GetFrom(), feePayer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch base input: %w", err)
	}

	stakingInput := &StakingInput{
		*txInput,
	}
	builder, err := NewTxBuilder(c.Asset.GetChain().Base())
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
	if ok {
		stakingInput.GasBudget = gasFee
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
	txInput, err := c.fetchBaseInput(ctx, NativeCoin, args.GetFrom(), feePayer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch base input: %w", err)
	}

	suiAddress, err := move_types.NewAccountAddressHex(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to encode adddress: %w", err)
	}

	rawStakes, err := c.SuiClient.GetStakes(ctx, *suiAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get stakes: %w", err)
	}

	validator, _ := args.GetValidator()
	account, _ := args.GetStakeAccount()
	stakeBalances := SuiStakesToUnstakeInputs(rawStakes, validator, account)

	stakesToClose := make([]Stake, 0)
	stakeToSplit := Stake{}
	splitRemainder := xc.NewAmountBlockchainFromUint64(0)
	// check if there is any stake which amount is equal to unstake amount
	// and use it as only input
	if ok, stakeObject := CanUnstakeAnyObject(stakeBalances, amount); ok {
		stakeId, err := HexToAddress(stakeObject.ObjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to decode stakeId: %w", err)
		}

		objectDetails, err := c.SuiClient.GetObject(ctx, sui_types.SuiAddress(stakeId), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get stake object details: %w", err)
		}
		if objectDetails == nil {
			return nil, errors.New("invalid get object response")
		}

		stakeObject.Version = objectDetails.Data.Version.Uint64()
		stakeObject.Digest = objectDetails.Data.Digest.String()

		stakesToClose = append(stakesToClose, *stakeObject)
	} else {
		// there is no single stake object that satisfies this case
		// we have to prepare a series of unstake/split/merge operations, depending on the amount

		// sort stakes in ascending order, we want to close as much staking accounts as possible
		slices.SortFunc(stakeBalances[:], func(lhs, rhs Stake) int {
			lhsBalance := lhs.GetBalance()
			rhsBalance := rhs.GetBalance()
			return lhsBalance.Cmp(&rhsBalance)
		})

		// split stakes into full unstakes and merge + split
		zeroAmount := xc.NewAmountBlockchainFromUint64(0)
		amountToUnstake := amount
		for _, s := range stakeBalances {
			// amount in UnstakingInput stakes are sufficient for unstake operation
			if amountToUnstake.Cmp(&zeroAmount) <= 0 {
				break
			}

			stakeId, err := HexToAddress(s.ObjectId)
			if err != nil {
				return nil, fmt.Errorf("failed to decode stake object id: %w", err)
			}

			objectDetails, err := c.SuiClient.GetObject(ctx, sui_types.SuiAddress(stakeId), nil)
			if err != nil {
				return nil, fmt.Errorf("failed to get stake object details: %w", err)
			}
			if objectDetails == nil {
				return nil, errors.New("invalid get object response")
			}

			s.Version = objectDetails.Data.Version.Uint64()
			s.Digest = objectDetails.Data.Digest.String()

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

	unstakingInput := &UnstakingInput{
		TxInput:         *txInput,
		StakesToUnstake: stakesToClose,
		StakeToSplit:    stakeToSplit,
		SplitAmount:     splitRemainder,
	}

	builder, err := NewTxBuilder(c.Asset.GetChain().Base())
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
	if ok {
		unstakingInput.GasBudget = gasFee
	}
	return unstakingInput, nil
}

// Check if given amount can be unstaked without spliting/merging any stakes
func CanUnstakeAnyObject(stakes []Stake, unstakeAmount xc.AmountBlockchain) (bool, *Stake) {
	for _, s := range stakes {
		balance := s.GetBalance()
		if balance.Cmp(&unstakeAmount) == 0 {
			return true, &s
		}
	}

	return false, nil
}

// Fetch input for a withdraw transaction -- not all chains use this as they combine it with unstake
func (c *Client) FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error) {
	return nil, nil
}

func SuiStakesToStakedBalances(stakes []types.DelegatedStake, validatorFilter string, accountFilter string) []*xclient.StakedBalance {
	stakedBalances := make([]*xclient.StakedBalance, 0)
	for _, stake := range stakes {
		validator := stake.ValidatorAddress
		if validatorFilter != "" && string(validatorFilter) != validator.String() {
			continue
		}

		for _, stakeState := range stake.Stakes {
			// initial stake value
			principalAmount := stakeState.Data.Principal
			xcStakeBalance := xc.NewAmountBlockchainFromUint64(principalAmount.Uint64())

			if stakeState.Data.StakeStatus == nil {
				logrus.WithField("staked_sui_id", stakeState.Data.StakedSuiId.String()).Warn("missing stake status")
				continue
			}

			account := stakeState.Data.StakedSuiId.String()
			if accountFilter != "" && account != string(accountFilter) {
				continue
			}

			stakeStatus := stakeState.Data.StakeStatus.Data
			var state xclient.StakeState
			if stakeStatus.Pending != nil {
				state = xclient.Activating
			} else if stakeStatus.Active != nil {
				state = xclient.Active
				rewards := stakeStatus.Active.EstimatedReward.Uint64()
				xcRewards := xc.NewAmountBlockchainFromUint64(rewards)
				xcStakeBalance = xcStakeBalance.Add(&xcRewards)
			} else if stakeStatus.Unstaked != nil {
				state = xclient.Inactive
			}

			stakedBalance := xclient.NewStakedBalance(xcStakeBalance, state, validator.String(), account)
			stakedBalances = append(stakedBalances, stakedBalance)
		}
	}

	return stakedBalances
}

func SuiStakesToUnstakeInputs(stakes []types.DelegatedStake, validatorFilter string, accountFilter string) []Stake {
	unstakeInputs := make([]Stake, 0)
	for _, stake := range stakes {
		validator := stake.ValidatorAddress
		if validatorFilter != "" && string(validatorFilter) != validator.String() {
			continue
		}

		for _, stakeState := range stake.Stakes {
			// initial stake value
			principalAmount := xc.NewAmountBlockchainFromUint64(stakeState.Data.Principal.Uint64())
			rewards := xc.NewAmountBlockchainFromUint64(0)

			if stakeState.Data.StakeStatus == nil {
				logrus.WithField("staked_sui_id", stakeState.Data.StakedSuiId.String()).Warn("missing stake status")
				continue
			}

			account := stakeState.Data.StakedSuiId.String()
			if accountFilter != "" && account != string(accountFilter) {
				continue
			}

			stakeStatus := stakeState.Data.StakeStatus.Data
			var state xclient.StakeState
			if stakeStatus.Pending != nil {
				state = xclient.Activating
			} else if stakeStatus.Active != nil {
				state = xclient.Active
				r := stakeStatus.Active.EstimatedReward.Uint64()
				rewards = xc.NewAmountBlockchainFromUint64(r)
			} else if stakeStatus.Unstaked != nil {
				state = xclient.Inactive
			}

			s := Stake{
				Principal: principalAmount,
				Rewards:   rewards,
				ObjectId:  account,
				State:     state,
				Validator: validator.String(),
			}
			unstakeInputs = append(unstakeInputs, s)
		}
	}

	return unstakeInputs
}
