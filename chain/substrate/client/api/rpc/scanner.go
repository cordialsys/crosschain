package rpc

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/retriever"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/state"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	substratetx "github.com/cordialsys/crosschain/chain/substrate/tx"
	"github.com/sirupsen/logrus"
	"github.com/vedhavyas/go-subkey/v2/scale"
	"golang.org/x/time/rate"
)

type Client struct {
	rpc      *gsrpc.SubstrateAPI
	maxDepth int
	limiter  *rate.Limiter
}

func NewClient(rpc *gsrpc.SubstrateAPI, maxDepth int, limiter *rate.Limiter) *Client {
	return &Client{rpc, maxDepth, limiter}
}

type Tx struct {
	BlockHash     types.Hash
	Block         *types.SignedBlock
	Events        []api.EventI
	Extrinsic     *types.Extrinsic
	ExtrinsicHash []byte
}

func (tx *Tx) Get(module, event string) (api.EventI, bool) {
	for _, ev := range tx.Events {
		if strings.EqualFold(ev.GetModule(), module) && strings.EqualFold(ev.GetEvent(), event) {
			return ev, true
		}
	}
	return nil, false
}

func (client *Client) GetTx(ctx context.Context, extrinsicHash string) (*Tx, error) {

	if height, offset, err := api.BlockAndOffset(extrinsicHash).Parse(); err == nil {
		blockHash, err := client.rpc.RPC.Chain.GetBlockHash(height)
		if err != nil {
			return nil, err
		}
		block, err := client.rpc.RPC.Chain.GetBlock(blockHash)
		if err != nil {
			return nil, err
		}
		if offset >= len(block.Block.Extrinsics) {
			return nil, fmt.Errorf("block does not contain extrinsic at offset %d", offset)
		}
		ext := block.Block.Extrinsics[offset]

		matchingEvents, err := client.GetEvents(ctx, blockHash, offset)
		if err != nil {
			return nil, err
		}
		return &Tx{
			BlockHash:     blockHash,
			Block:         block,
			Events:        matchingEvents,
			Extrinsic:     &ext,
			ExtrinsicHash: HashExtrinsic(&ext),
		}, nil
	} else {
		extrinsicHashBz, err := codec.HexDecodeString(extrinsicHash)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction hash: %v", err)
		}
		block, blockHash, ext, offset, found, err := client.ScanBlocksForExtrinsic(ctx, extrinsicHashBz)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction hash: %v", err)
		}
		if !found {
			return nil, fmt.Errorf("could not find extrinsic by hash %s in the last %d blocks", extrinsicHash, client.maxDepth)
		}
		extId := fmt.Sprintf("%d-%d", block.Block.Header.Number, offset)
		logrus.WithField("id", extId).Debug("found extrinsic")
		client.GetEvents(ctx, blockHash, offset)
		matchingEvents, err := client.GetEvents(ctx, blockHash, offset)
		if err != nil {
			return nil, err
		}
		return &Tx{
			BlockHash:     blockHash,
			Block:         block,
			Events:        matchingEvents,
			ExtrinsicHash: extrinsicHashBz,
			Extrinsic:     ext,
		}, nil
	}

}

func (client *Client) GetEvents(ctx context.Context, blockHash types.Hash, extrinsicOffset int) ([]api.EventI, error) {
	matchingEvents := []api.EventI{}
	retriever, err := retriever.NewDefaultEventRetriever(state.NewEventProvider(client.rpc.RPC.State), client.rpc.RPC.State)
	if err != nil {
		return matchingEvents, err
	}

	events, err := retriever.GetEvents(blockHash)
	if err != nil {
		return matchingEvents, err
	}
	for _, event := range events {
		if event.Phase.AsApplyExtrinsic == uint32(extrinsicOffset) {
			matchingEvents = append(matchingEvents, NewEvent(event))
		}
	}
	return matchingEvents, nil
}

func (client *Client) ScanBlocksForExtrinsic(ctx context.Context, extrinsicHash []byte) (block *types.SignedBlock, blockHash types.Hash, ext *types.Extrinsic, index int, found bool, err error) {
	// get the first block
	header, err := client.rpc.RPC.Chain.GetHeaderLatest()
	if err != nil {
		return nil, blockHash, ext, -1, false, err
	}
	blockHash, err = client.rpc.RPC.Chain.GetBlockHash(uint64(header.Number))
	if err != nil {
		return nil, blockHash, ext, -1, false, err
	}
	scans := 0
	for scans < client.maxDepth {
		client.limiter.Wait(ctx)
		scans++
		block, err = client.rpc.RPC.Chain.GetBlock(blockHash)
		if err != nil {
			return nil, blockHash, ext, -1, false, err
		}
		// fmt.Println("-- block", block.Block.Header.Number)
		index = -1
		for i, ext := range block.Block.Extrinsics {
			if bytes.Equal(extrinsicHash, HashExtrinsic(&ext)) {
				return block, blockHash, &ext, i, true, nil
			}
		}
		// scan the parent next
		blockHash = block.Block.Header.ParentHash
	}
	// not found
	return nil, blockHash, ext, -1, false, nil
}

func HashExtrinsic(ext *types.Extrinsic) []byte {
	// Force version to be treated as v4.
	// Some extrinisics are v5 but are still compatible when hashing.
	// This implementation is copied from the library, ignores the version.
	// Better than _always_failing to hash.
	v4 := v4Extrinsic{ext}
	bz, err := codec.Encode(&v4)
	if err != nil {
		logrus.WithError(err).Error("failed to encode extrinsic")
	}
	return substratetx.HashSerialized(bz)
}

type v4Extrinsic struct {
	*types.Extrinsic
}

func (e v4Extrinsic) Encode(encoder scale.Encoder) error {
	if e.Type() != types.ExtrinsicVersion4 {
		// ignore
	}
	var bb = bytes.Buffer{}
	tempEnc := scale.NewEncoder(&bb)

	err := tempEnc.Encode(e.Version)
	if err != nil {
		return err
	}

	if e.IsSigned() {
		err = tempEnc.Encode(e.Signature)
		if err != nil {
			return err
		}
	}

	err = tempEnc.Encode(e.Method)
	if err != nil {
		return err
	}

	eb := bb.Bytes()
	err = encoder.EncodeUintCompact(*big.NewInt(0).SetUint64(uint64(len(eb))))
	if err != nil {
		return err
	}

	err = encoder.Write(eb)
	if err != nil {
		return err
	}

	return nil
}
