package client

import (
	"bytes"
	"context"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// NominationPoolsStakingClient handles staking operations for substrate chains using nomination pools
type NominationPoolsStakingClient struct {
	client *Client
}

// FetchStakeBalance for substrate chains using nomination pools
func (pools *NominationPoolsStakingClient) FetchStakeBalance(_ context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	meta, err := pools.client.DotClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	addrBz, err := address.Decode(args.GetFrom())
	if err != nil {
		return nil, err
	}

	// Query PoolMembers storage to get member info
	key, err := types.CreateStorageKey(meta, "NominationPools", "PoolMembers", addrBz.ToBytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %v", err)
	}

	// PoolMember struct from substrate nomination pools pallet
	// See: https://github.com/paritytech/substrate/blob/master/frame/nomination-pools/src/lib.rs
	// The actual struct has: pool_id, points, last_recorded_reward_counter, unbonding_eras
	// Where unbonding_eras is a BoundedBTreeMap<EraIndex, Balance> encoded as a vec of pairs

	// Get raw data to decode the full struct
	var rawData types.StorageDataRaw
	ok, err := pools.client.DotClient.RPC.State.GetStorageLatest(key, &rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to query pool member: %v", err)
	}

	if !ok {
		// No staked balance found
		return []*xclient.StakedBalance{}, nil
	}

	// Decode the full struct including unbonding_eras
	var fullPoolMember struct {
		PoolId                    types.U32
		Points                    types.U128
		LastRecordedRewardCounter types.U128
		UnbondingEras             []struct {
			Era    types.U32
			Amount types.U128
		}
	}

	decoder := scale.NewDecoder(bytes.NewReader(rawData))
	err = decoder.Decode(&fullPoolMember)
	if err != nil {
		return nil, fmt.Errorf("failed to decode pool member: %v", err)
	}

	poolIdStr := fmt.Sprintf("%d", fullPoolMember.PoolId)
	balances := xclient.StakedBalanceState{}

	// Active balance (bonded points)
	balances.Active = xc.AmountBlockchain(*fullPoolMember.Points.Int)

	// Unbonding balance (sum of all unbonding eras)
	if len(fullPoolMember.UnbondingEras) > 0 {
		total := xc.NewAmountBlockchainFromUint64(0)
		for _, unbonding := range fullPoolMember.UnbondingEras {
			if unbonding.Amount.Int.Uint64() > 0 {
				toAdd := xc.AmountBlockchain(*unbonding.Amount.Int)
				total = total.Add(&toAdd)
			}
		}
		if total.Uint64() > 0 {
			balances.Deactivating = total
		}
	}

	return []*xclient.StakedBalance{
		xclient.NewStakedBalances(
			balances,
			poolIdStr, // pool ID serves as validator identifier
			"",
		),
	}, nil
}

func (pools *NominationPoolsStakingClient) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	chainCfg := pools.client.Asset.GetChain().Base()
	amount, ok := args.GetAmount()
	if !ok {
		return nil, buildererrors.ErrStakingAmountRequired
	}
	tfArgs, _ := xcbuilder.NewTransferArgs(chainCfg, args.GetFrom(), "", amount)
	input, err := pools.client.FetchTransferInput(ctx, tfArgs)
	if err != nil {
		return nil, err
	}
	stakingInput := input.(*tx_input.TxInput)

	// Check if account already joined a pool
	meta, err := pools.client.DotClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	addrBz, err := address.Decode(args.GetFrom())
	if err != nil {
		return nil, err
	}
	validator, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("must provide validator / pool ID")
	}

	// Query PoolMembers storage to see if we are already in a pool
	key, err := types.CreateStorageKey(meta, "NominationPools", "PoolMembers", addrBz.ToBytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %v", err)
	}

	var rawData types.StorageDataRaw
	ok, err = pools.client.DotClient.RPC.State.GetStorageLatest(key, &rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to query pool member: %v", err)
	}

	if ok {
		// Account is already in a pool, decode to get pool ID
		var poolMember struct {
			PoolId                    types.U32
			Points                    types.U128
			LastRecordedRewardCounter types.U128
		}

		decoder := scale.NewDecoder(bytes.NewReader(rawData))
		err = decoder.Decode(&poolMember)
		if err != nil {
			return nil, fmt.Errorf("failed to decode pool member: %v", err)
		}

		stakingInput.AlreadyJoinedPool = true
		stakingInput.JoinedPoolId = uint32(poolMember.PoolId)

		// Substrate does not allow staking to separate pools.
		// Once you join a pool, then subsequent stake increases will go only to that pool.
		if validator != fmt.Sprintf("%d", poolMember.PoolId) {
			return nil, fmt.Errorf("already joined pool %d, cannot join separate pool %s", poolMember.PoolId, validator)
		}
	}

	return stakingInput, nil
}
