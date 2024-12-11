package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// Fetch staked balances accross different possible states
func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	return nil, fmt.Errorf("not implemented")
}

// Can use the normal tx-input
func (client *Client) FetchStakingInput(ctx context.Context, args builder.StakeArgs) (xc.StakeTxInput, error) {
	tfArgs, _ := xcbuilder.NewTransferArgs(args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}

// Can use the normal tx-input
func (client *Client) FetchUnstakingInput(ctx context.Context, args builder.StakeArgs) (xc.UnstakeTxInput, error) {
	tfArgs, _ := xcbuilder.NewTransferArgs(args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}

// Can use the normal tx-input
func (client *Client) FetchWithdrawInput(ctx context.Context, args builder.StakeArgs) (xc.WithdrawTxInput, error) {
	tfArgs, _ := xcbuilder.NewTransferArgs(args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}
