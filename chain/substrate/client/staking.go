package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/client/api/taostats"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// Fetch staked balances accross different possible states
func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	if client.Asset.GetChain().Chain == xc.TAO {
		if client.Asset.GetChain().IndexerType != IndexerTaostats {
			return nil, fmt.Errorf("client not configured to get TAO staked balances")
		}

		taoStatsClient := taostats.NewClient(client.indexerUrl, client.apiKey, client.Asset.GetChain().Limiter)
		account, err := taoStatsClient.GetAccount(ctx, string(args.GetFrom()))
		if err != nil {
			return nil, err
		}

		return []*xclient.StakedBalance{
			xclient.NewStakedBalance(
				xc.NewAmountBlockchainFromStr(account.BalanceStaked),
				xclient.Active,
				// unfortunately, cannot get validator + netuid
				"",
				"",
			),
		}, nil
		// Not longer works:
		// TAO now has a much more complex way to query staked balances.  Using TAOStats API instead for now.
		// meta, err := client.DotClient.RPC.State.GetMetadataLatest()
		// if err != nil {
		// 	return nil, err
		// }

		// key, err := types.CreateStorageKey(meta, "SubtensorModule", "TotalColdkeyStake", base58.Decode(string(args.GetFrom()))[1:33])
		// if err != nil {
		// 	return nil, err
		// }

		// var bal types.U64
		// ok, err := client.DotClient.RPC.State.GetStorageLatest(key, &bal)
		// if err != nil || !ok {
		// 	return nil, err
		// }

		// return []*xclient.StakedBalance{
		// 	xclient.NewStakedBalance(xc.NewAmountBlockchainFromUint64(uint64(bal)), xclient.Active, "", ""),
		// }, nil
	}

	return nil, fmt.Errorf("not implemented for generic substrate chains")
}

// Can use the normal tx-input
func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}

// Can use the normal tx-input
func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}

// Can use the normal tx-input
func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", args.GetAmount())
	input, err := client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakinginput := input.(*tx_input.TxInput)
	return stakinginput, nil
}
