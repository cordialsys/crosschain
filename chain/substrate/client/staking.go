package client

import (
	"context"
	"errors"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// Fetch staked balances across different possible states
func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	if client.Asset.GetChain().Chain == xc.TAO {
		taoClient := &TaoStakingClient{client: client}
		return taoClient.FetchStakeBalance(ctx, args)
	}

	// For generic substrate chains using nomination pools
	poolsClient := &NominationPoolsStakingClient{client: client}
	return poolsClient.FetchStakeBalance(ctx, args)
}

// Fetch staking input
func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	if client.Asset.GetChain().Chain == xc.TAO {
		taoClient := &TaoStakingClient{client: client}
		return taoClient.FetchStakingInput(ctx, args)
	}

	// For generic substrate chains using nomination pools
	poolsClient := &NominationPoolsStakingClient{client: client}
	return poolsClient.FetchStakingInput(ctx, args)
}

// Can use the normal tx-input
func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	chainCfg := client.Asset.GetChain().Base()

	amount, ok := args.GetAmount()
	if !ok {
		return nil, errors.New("missing staking amount")
	}

	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", amount)
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakingInput := input.(*tx_input.TxInput)
	return stakingInput, nil
}

// Can use the normal tx-input
func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	amount, ok := args.GetAmount()
	if !ok {
		return nil, errors.New("missing staking amount")
	}
	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", amount)
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakingInput := input.(*tx_input.TxInput)
	return stakingInput, nil
}
