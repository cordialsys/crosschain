package client

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// Fetch staked balances accross different possible states
func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	if client.Asset.GetChain().Chain == xc.TAO {
		// zero := xc.NewAmountBlockchainFromUint64(0)
		meta, err := client.DotClient.RPC.State.GetMetadataLatest()
		if err != nil {
			return nil, err
		}

		key, err := types.CreateStorageKey(meta, "SubtensorModule", "TotalColdkeyStake", base58.Decode(string(args.GetFrom()))[1:33])
		if err != nil {
			return nil, err
		}

		var bal types.U64
		ok, err := client.DotClient.RPC.State.GetStorageLatest(key, &bal)
		if err != nil || !ok {
			return nil, err
		}

		return []*xclient.StakedBalance{
			xclient.NewStakedBalance(xc.NewAmountBlockchainFromUint64(uint64(bal)), xclient.Active, "", ""),
		}, nil
	}

	return nil, fmt.Errorf("not implemented for generic substrate chains")
}

// Can use the normal tx-input
func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	tfArgs, _ := xcbuilder.NewTransferArgs(args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}

// Can use the normal tx-input
func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	tfArgs, _ := xcbuilder.NewTransferArgs(args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}

// Can use the normal tx-input
func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	tfArgs, _ := xcbuilder.NewTransferArgs(args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}
