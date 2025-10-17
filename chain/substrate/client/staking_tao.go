package client

import (
	"context"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/client/api/taostats"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// TaoStakingClient handles staking operations for Bittensor (TAO) chain
type TaoStakingClient struct {
	client *Client
}

// FetchStakeBalance for TAO chain using TaoStats API
func (tao *TaoStakingClient) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	if tao.client.Asset.GetChain().IndexerType != IndexerTaostats {
		return nil, fmt.Errorf("client not configured to get TAO staked balances")
	}

	taoStatsClient := taostats.NewClient(tao.client.indexerUrl, tao.client.apiKey, tao.client.Asset.GetChain().Limiter)
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
	// meta, err := tao.client.DotClient.RPC.State.GetMetadataLatest()
	// if err != nil {
	// 	return nil, err
	// }

	// key, err := types.CreateStorageKey(meta, "SubtensorModule", "TotalColdkeyStake", base58.Decode(string(args.GetFrom()))[1:33])
	// if err != nil {
	// 	return nil, err
	// }

	// var bal types.U64
	// ok, err := tao.client.DotClient.RPC.State.GetStorageLatest(key, &bal)
	// if err != nil || !ok {
	// 	return nil, err
	// }

	// return []*xclient.StakedBalance{
	// 	xclient.NewStakedBalance(xc.NewAmountBlockchainFromUint64(uint64(bal)), xclient.Active, "", ""),
	// }, nil
}

// FetchStakingInput for TAO - uses normal tx-input
func (tao *TaoStakingClient) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	chainCfg := tao.client.Asset.GetChain().Base()
	amount, ok := args.GetAmount()
	if !ok {
		return nil, errors.New("missing stake amount")
	}
	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", amount)
	input, err := tao.client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakingInput := input.(*tx_input.TxInput)
	return stakingInput, nil
}
