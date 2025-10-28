package tron

import (
	"context"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	builder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/tron/common"
	"github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	xclient "github.com/cordialsys/crosschain/client"
)

const (
	TRON_BANDWIDTH     = "TRON_BANDWIDTH"
	TRON_ENERGY        = "TRON_ENERGY"
	TRON_POWER         = "TRON_POWER"
	FREEZE_BALANCEV2   = "freezebalancev2"
	RESOURCE_BANDWIDTH = "BANDWIDTH"
	RESOURCE_ENERGY    = "ENERGY"
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
	totalVotes := common.TrxToVotes(frozenBalance, decimals)
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
	freezeInput, err := c.FetchBaseInput(ctx, map[string]any{
		"owner_address":  string(args.GetFrom()),
		"frozen_balance": common.VotesToTrx(1, int(c.chain.Decimals)).Uint64(),
		"resource":       RESOURCE_BANDWIDTH,
	}, "freezebalancev2")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	// use 'freezebalancev2' for vote input - the operation is not important, and we
	// could fail in case the account never frozen any TRX in the past
	voteInput, err := c.FetchBaseInput(ctx, map[string]any{
		"owner_address":  string(args.GetFrom()),
		"frozen_balance": common.VotesToTrx(1, int(c.chain.Decimals)).Uint64(),
		"resource":       RESOURCE_BANDWIDTH,
	}, "freezebalancev2")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	account, err := c.client.GetAccount(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	stakeInput := txinput.StakeInput{
		TxInput:        *freezeInput,
		VoteInput:      *voteInput,
		Votes:          account.Votes,
		FreezedBalance: account.GetFrozenBalance(),
		Decimals:       int(c.chain.Decimals),
	}

	return &stakeInput, nil
}

func (c Client) FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error) {
	// use 'unfreezebalancev2' for vote input - the operation is not important, and we
	// could fail in case the account never voted in the past
	voteInput, err := c.FetchBaseInput(ctx, map[string]any{
		"owner_address":    string(args.GetFrom()),
		"unfreeze_balance": common.VotesToTrx(1, int(c.chain.Decimals)).Uint64(),
		"resource":         RESOURCE_BANDWIDTH,
	}, "unfreezebalancev2")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	unfreezeInput, err := c.FetchBaseInput(ctx, map[string]any{
		"owner_address":    string(args.GetFrom()),
		"unfreeze_balance": common.VotesToTrx(1, int(c.chain.Decimals)).Uint64(),
		"resource":         RESOURCE_BANDWIDTH,
	}, "unfreezebalancev2")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
	}

	account, err := c.client.GetAccount(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	return &txinput.UnstakeInput{
		TxInput:        *voteInput,
		UnfreezeInput:  *unfreezeInput,
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

	var unfrozeBalanceInput *txinput.TxInput
	if unfrozenToWithdraw {
		input, err := c.FetchBaseInput(ctx, map[string]any{
			"owner_address": string(args.GetFrom()),
		}, "withdrawexpireunfreeze")
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToFetchBaseInput, err)
		}
		unfrozeBalanceInput = input
	}

	var getRewardInput *txinput.TxInput
	if account.Allowance > 0 {
		input, err := c.FetchBaseInput(ctx, map[string]any{
			"owner_address": string(args.GetFrom()),
		}, "withdrawbalance")
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
