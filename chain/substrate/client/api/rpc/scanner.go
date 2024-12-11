package rpc

import (
	"bytes"
	"context"
	"fmt"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/retriever"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/state"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	substratetx "github.com/cordialsys/crosschain/chain/substrate/tx"
	"github.com/sirupsen/logrus"
)

type Client struct {
	rpc      *gsrpc.SubstrateAPI
	maxDepth int
}

func NewClient(rpc *gsrpc.SubstrateAPI, maxDepth int) *Client {
	return &Client{rpc, maxDepth}
}

func (client *Client) GetTx(ctx context.Context, extrinsicHash string) (interface{}, error) {
	if height, offset, err := api.BlockAndOffset(extrinsicHash).Parse(); err == nil {
		blockHash, err := client.rpc.RPC.Chain.GetBlockHash(height)
		if err != nil {
			// return nil, blockHash, -1, false, err
			return nil, err
		}
		client.GetEvents(ctx, blockHash, offset)
	} else {
		extrinsicHashBz, err := codec.HexDecodeString(extrinsicHash)
		if err != nil {
			return xc.LegacyTxInfo{}, fmt.Errorf("invalid transaction hash: %v", err)
		}
		block, hash, offset, found, err := client.ScanBlocksForExtrinsic(ctx, extrinsicHashBz)
		if err != nil {
			return xc.LegacyTxInfo{}, fmt.Errorf("invalid transaction hash: %v", err)
		}
		if !found {
			return xc.LegacyTxInfo{}, fmt.Errorf("could not find extrinsic by hash %s in the last %d blocks", extrinsicHash, client.maxDepth)
		}
		extId := fmt.Sprintf("%d-%d", block.Block.Header.Number, offset)
		logrus.WithField("id", extId).Debug("found extrinsic")
		client.GetEvents(ctx, hash, offset)
	}
	// TODO
	return nil, nil
}

func (client *Client) GetEvents(ctx context.Context, blockHash types.Hash, extrinsicOffset int) (interface{}, error) {
	// Use RPC
	retriever, err := retriever.NewDefaultEventRetriever(state.NewEventProvider(client.rpc.RPC.State), client.rpc.RPC.State)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	events, err := retriever.GetEvents(blockHash)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	for _, event := range events {
		if event.Phase.AsApplyExtrinsic == uint32(extrinsicOffset) {
			fmt.Printf("Event ID: %x \n", event.EventID)
			fmt.Printf("Event Name: %s \n", event.Name)
			fmt.Printf("Event Fields Count: %d \n", len(event.Fields))

			for _, v := range event.Fields {
				fmt.Printf("  Field Type: %T \n", v)
				fmt.Printf("  Field Value: %v \n", v)
			}
		} else {

			fmt.Printf("SKIP: %s \n", event.Name)
		}
	}
	// TODO
	return nil, nil
}

func (client *Client) ScanBlocksForExtrinsic(ctx context.Context, extrinsicHash []byte) (block *types.SignedBlock, blockHash types.Hash, index int, found bool, err error) {
	// get the first block
	header, err := client.rpc.RPC.Chain.GetHeaderLatest()
	if err != nil {
		return nil, blockHash, -1, false, err
	}
	blockHash, err = client.rpc.RPC.Chain.GetBlockHash(uint64(header.Number))
	if err != nil {
		return nil, blockHash, -1, false, err
	}
	scans := 0
	for scans < client.maxDepth {
		scans++
		block, err = client.rpc.RPC.Chain.GetBlock(blockHash)
		if err != nil {
			return nil, blockHash, -1, false, err
		}
		// fmt.Println("-- block", block.Block.Header.Number)
		index = -1
		for i, ext := range block.Block.Extrinsics {
			bz, err := codec.Encode(ext)
			if err != nil {
				return nil, blockHash, -1, false, fmt.Errorf("bad extrinisic %d-%d: %v", block.Block.Header.Number, i, err)
			}
			maybeHash := substratetx.HashSerialized(bz)
			// fmt.Println("  --", i, "", hex.EncodeToString(maybeHash))
			if bytes.Equal(extrinsicHash, maybeHash) {
				return block, blockHash, i, true, nil
			}
		}
		// scan the parent next
		blockHash = block.Block.Header.ParentHash
	}
	// not found
	return nil, blockHash, -1, false, nil
}
