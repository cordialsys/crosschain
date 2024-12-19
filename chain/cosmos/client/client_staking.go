package client

import (
	"context"
	"time"

	stakingtypes "cosmossdk.io/x/staking/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cosmos/cosmos-sdk/types/query"
)

func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	q := stakingtypes.NewQueryClient(client.Ctx)
	delegations, err := q.DelegatorDelegations(ctx, &stakingtypes.QueryDelegatorDelegationsRequest{
		DelegatorAddr: string(args.GetFrom()),
		Pagination: &query.PageRequest{
			Limit: 1000,
		},
	})
	if err != nil {
		return nil, err
	}

	unbonding, err := q.DelegatorUnbondingDelegations(ctx, &stakingtypes.QueryDelegatorUnbondingDelegationsRequest{
		DelegatorAddr: string(args.GetFrom()),
		Pagination: &query.PageRequest{
			Limit: 1000,
		},
	})
	if err != nil {
		return nil, err
	}

	balances := []*xclient.StakedBalance{}
	for _, bal := range delegations.DelegationResponses {
		balances = append(balances, xclient.NewStakedBalance(
			xc.AmountBlockchain(*bal.Balance.Amount.BigInt()),
			xclient.Active,
			bal.Delegation.ValidatorAddress,
			"",
		))
	}
	for _, bal := range unbonding.UnbondingResponses {
		for _, entry := range bal.Entries {
			state := xclient.Deactivating
			if time.Since(entry.CompletionTime) > 0 {
				state = xclient.Inactive
			}
			amount := xc.AmountBlockchain(*entry.Balance.BigInt())
			balances = append(balances, xclient.NewStakedBalance(
				amount,
				state,
				bal.ValidatorAddress,
				"",
			))
		}
	}
	return balances, nil
}

func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	baseTxInput, err := client.FetchBaseTxInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	return &tx_input.StakingInput{
		TxInput: *baseTxInput,
	}, nil
}

func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	baseTxInput, err := client.FetchBaseTxInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	return &tx_input.UnstakingInput{
		TxInput: *baseTxInput,
	}, nil
}

func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	baseTxInput, err := client.FetchBaseTxInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	return &tx_input.WithdrawInput{
		TxInput: *baseTxInput,
	}, nil
}
