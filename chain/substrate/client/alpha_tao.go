package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
)

// FetchAlphaBalance queries all alpha token positions for a coldkey on a given subnet
// and returns the total balance summed across all hotkeys.
func (client *Client) FetchAlphaBalance(ctx context.Context, addr xc.Address, contract string) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	netuid, err := substrate.ParseAlphaContract(contract)
	if err != nil {
		return zero, err
	}

	positions, err := client.fetchAlphaPositions(addr, netuid)
	if err != nil {
		return zero, err
	}

	var total uint64
	for _, pos := range positions {
		total += pos.Amount
	}

	return xc.NewAmountBlockchainFromUint64(total), nil
}

// FetchAlphaPositions queries all alpha positions for a coldkey on a specific subnet.
// It first looks up all hotkeys the coldkey stakes with, then queries Alpha balance for each.
func (client *Client) fetchAlphaPositions(addr xc.Address, netuid uint16) ([]tx_input.AlphaPosition, error) {
	meta, err := client.DotClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %v", err)
	}

	coldkeyAddr, err := address.Decode(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid coldkey address: %v", err)
	}

	// Query StakingHotkeys(coldkey) -> Vec<AccountID>
	hotkeys, err := client.fetchStakingHotkeys(meta, coldkeyAddr)
	if err != nil {
		return nil, err
	}

	netuidBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(netuidBytes, netuid)

	var positions []tx_input.AlphaPosition
	for _, hotkey := range hotkeys {
		balance, err := client.fetchSingleAlphaBalance(meta, &hotkey, coldkeyAddr, netuidBytes)
		if err != nil {
			return nil, err
		}
		if balance > 0 {
			// Encode hotkey back to SS58 address
			hotkeyAddr, err := address.NewAddressBuilder(client.Asset.GetChain().Base())
			if err != nil {
				return nil, fmt.Errorf("failed to create address builder: %v", err)
			}
			hotkeyStr, err := hotkeyAddr.GetAddressFromPublicKey(hotkey.ToBytes())
			if err != nil {
				return nil, fmt.Errorf("failed to encode hotkey address: %v", err)
			}
			positions = append(positions, tx_input.AlphaPosition{
				Hotkey: string(hotkeyStr),
				Amount: balance,
			})
		}
	}

	// Sort by amount descending so UTXO selection takes biggest positions first
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].Amount > positions[j].Amount
	})

	return positions, nil
}

// fetchStakingHotkeys queries SubtensorModule.StakingHotkeys(coldkey) -> Vec<AccountID>
func (client *Client) fetchStakingHotkeys(meta *types.Metadata, coldkey *types.AccountID) ([]types.AccountID, error) {
	key, err := types.CreateStorageKey(meta, "SubtensorModule", "StakingHotkeys", coldkey.ToBytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key for StakingHotkeys: %v", err)
	}

	var rawData types.StorageDataRaw
	ok, err := client.DotClient.RPC.State.GetStorageLatest(key, &rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to query StakingHotkeys: %v", err)
	}
	if !ok || len(rawData) == 0 {
		return nil, nil
	}

	// Decode Vec<AccountID> using SCALE codec
	decoder := scale.NewDecoder(bytes.NewReader(rawData))
	// Vec prefix: compact-encoded length
	compactLen := types.UCompact{}
	err = decoder.Decode(&compactLen)
	if err != nil {
		return nil, fmt.Errorf("failed to decode StakingHotkeys length: %v", err)
	}
	count := compactLen.Int64()

	hotkeys := make([]types.AccountID, count)
	for i := int64(0); i < count; i++ {
		err = decoder.Decode(&hotkeys[i])
		if err != nil {
			return nil, fmt.Errorf("failed to decode hotkey at index %d: %v", i, err)
		}
	}

	return hotkeys, nil
}

// fetchSingleAlphaBalance queries SubtensorModule.Alpha(hotkey, coldkey, netuid) -> u64
func (client *Client) fetchSingleAlphaBalance(meta *types.Metadata, hotkey, coldkey *types.AccountID, netuidBytes []byte) (uint64, error) {
	key, err := types.CreateStorageKey(meta, "SubtensorModule", "Alpha", hotkey.ToBytes(), coldkey.ToBytes(), netuidBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to create storage key for Alpha: %v", err)
	}

	var alphaBalance types.U64
	ok, err := client.DotClient.RPC.State.GetStorageLatest(key, &alphaBalance)
	if err != nil {
		return 0, fmt.Errorf("failed to query Alpha balance: %v", err)
	}
	if !ok {
		return 0, nil
	}

	return uint64(alphaBalance), nil
}
