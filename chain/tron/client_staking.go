package tron

import (
	"context"
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	builder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/tron/common"
	"github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	xclient "github.com/cordialsys/crosschain/client"
)

const (
	ERR_CONTRACT_VALIDATE = "ContractValidateException"
)

var _ xclient.StakingClient = &Client{}

func (c Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	resp, err := c.client.GetAccount(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	frozenBalance := xc.NewAmountBlockchainFromUint64(resp.GetFrozenBalance())
	decimals := int(c.chain.Decimals)
	balances := make([]*xclient.StakedBalance, 0)
	totalVotes, err := common.TrxToVotes(frozenBalance, decimals)
	if err != nil {
		return nil, ErrFailedTrxToVotesConversion
	}
	usedVotes := uint64(0)
	for _, v := range resp.Votes {
		usedVotes += v.VoteCount
		trx := common.VotesToTrx(v.VoteCount, decimals)
		balances = append(balances, xclient.NewStakedBalance(
			trx,
			xclient.Active,
			v.VoteAddress,
			"",
		))
	}

	unfrozenBalance := uint64(0)
	for _, uf := range resp.UnfrozenV2 {
		unfrozenBalance += uf.UnfreezeAmount
	}

	if unfrozenBalance > 0 {
		uf := xc.NewAmountBlockchainFromUint64(unfrozenBalance)
		balances = append(balances, xclient.NewStakedBalance(
			uf,
			xclient.Deactivating,
			"",
			"",
		))
	}

	unusedVotes := totalVotes - usedVotes
	if unusedVotes > 0 {
		unusedStakeBalance := common.VotesToTrx(unusedVotes, decimals)
		balances = append(balances, xclient.NewStakedBalance(
			unusedStakeBalance,
			xclient.Inactive,
			"",
			"",
		))
	}

	return balances, nil
}

func BuildVotes(votes []*httpclient.Vote) []map[string]any {
	mapVotes := make([]map[string]any, 0)

	// use vote_count of 1 - it doesn't affect tx size, and allows us to prepare the input aot
	for _, v := range votes {
		mapVotes = append(mapVotes, map[string]any{
			"vote_address": v.VoteAddress,
			"vote_count":   1,
		})
	}

	return mapVotes
}

func (c Client) FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error) {
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	freezeParams := httpclient.FreezeBalanceV2Params{
		Owner:         args.GetFrom(),
		FrozenBalance: amount,
	}
	freezeInput, err := c.FetchBaseInput(ctx, freezeParams)
	if AllowContractValidateError(err) != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	account, err := c.client.GetAccount(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	// use 'freezebalancev2' for vote input - the operation is not important, and we
	// could fail in case the account never frozen any TRX in the past
	voteInput, err := c.FetchBaseInput(ctx, freezeParams)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	stakeInput := txinput.StakeInput{
		TxInput:        *voteInput,
		FreezeInput:    freezeInput,
		Votes:          account.Votes,
		FreezedBalance: account.GetFrozenBalance(),
		Decimals:       int(c.chain.Decimals),
	}

	return &stakeInput, nil
}

func (c Client) FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error) {
	// use 'unfreezebalancev2' for vote input - the operation is not important, and we
	// could fail in case the account never voted in the past
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}

	unfreezeParams := httpclient.UnfreezeBalanceV2Params{
		Owner:           args.GetFrom(),
		UnfreezeBalance: amount,
	}
	voteInput, err := c.FetchBaseInput(ctx, &unfreezeParams)
	if AllowContractValidateError(err) != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	account, err := c.client.GetAccount(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}
	voteParams := httpclient.VoteWitnessAccountParams{
		Owner: args.GetFrom(),
		Votes: account.Votes,
	}
	unfreezeInput, err := c.FetchBaseInput(ctx, voteParams)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	return &txinput.UnstakeInput{
		TxInput:        *unfreezeInput,
		VoteInput:      voteInput,
		FreezedBalance: account.GetFrozenBalance(),
		Votes:          account.Votes,
		Decimals:       int(c.chain.Decimals),
	}, nil
}

func (c Client) FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error) {
	account, err := c.client.GetAccount(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	unfrozenToWithdraw := false
	now := time.Now().UnixMilli()
	for _, u := range account.UnfrozenV2 {
		if u.UnfreezeExpireTime < now {
			unfrozenToWithdraw = true
			break
		}
	}

	withdrawUnfreezeParams := httpclient.WithdrawExpiredUnfreezeParams{
		Owner: args.GetFrom(),
	}
	var unfrozeBalanceInput *txinput.TxInput
	if unfrozenToWithdraw {
		input, err := c.FetchBaseInput(ctx, withdrawUnfreezeParams)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
		}
		unfrozeBalanceInput = input
	}

	var getRewardInput *txinput.TxInput
	if account.Allowance > 0 {
		withdrawBalanceParams := httpclient.WithdrawBalanceParams{
			Owner: args.GetFrom(),
		}
		input, err := c.FetchBaseInput(ctx, withdrawBalanceParams)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
		}
		getRewardInput = input
	}

	return &txinput.WithdrawInput{
		TxInput:              unfrozeBalanceInput,
		WithdrawRewardsInput: getRewardInput,
	}, nil
}

// AllowContractValidateError allows the "ContractValidateException" - this means that the operation
// is not possible due to account status. However, for stake/unstake/withdrawals it means that
// part of the operation can be skipped.
// Example FetchStakingInput:
// 0. Account state: { balance: 0.6, frozen_balance: 25.0, available_votes: 25 }
// 1. User requests to stake 20TRX
// 2. Try to fetch input for FreezeBalanceV2 - it fails due to insufficient balance
// 3. Fetch input for VoteWitnessAccount - it succeeds - user has some free votes
// 4. We can properly stake, even if fetching FreezeInput failed
func AllowContractValidateError(err error) error {
	if err != nil && !strings.Contains(err.Error(), ERR_CONTRACT_VALIDATE) {
		return err
	}

	return nil
}
