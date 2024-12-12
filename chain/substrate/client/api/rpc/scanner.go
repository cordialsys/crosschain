package rpc

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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

type Tx struct {
	BlockHash     types.Hash
	Block         *types.SignedBlock
	Events        []api.EventI
	Extrinsic     *types.Extrinsic
	ExtrinsicHash []byte
}

func (tx *Tx) Failure() (string, bool) {
	for _, ev := range tx.Events {
		if strings.EqualFold(ev.GetModule(), "systems") && strings.EqualFold(ev.GetEvent(), "extrinsicfailure") {
			reason, ok := ev.GetParam("", 0)
			if ok {
				asString, ok := reason.(string)
				if ok {
					return asString, true
				} else {
					logrus.WithField("type", fmt.Sprintf("%T", reason)).Warn("did not expect type for failure")
				}
			}
		}
	}
	return "", false
}

func (tx *Tx) Fee(ab xc.AddressBuilder) (xc.Address, xc.AmountBlockchain, error) {
	for _, ev := range tx.Events {
		if strings.EqualFold(ev.GetModule(), "TransactionPayment") && strings.EqualFold(ev.GetEvent(), "TransactionFeePaid") {
			who, ok := ev.GetParam("", 0)
			if !ok {
				return "", xc.AmountBlockchain{}, fmt.Errorf("TransactionPayment.TransactionFeePaid did not have 0 param")
			}
			whoString, ok := who.(string)
			if !ok {
				return "", xc.AmountBlockchain{}, fmt.Errorf("TransactionPayment.TransactionFeePaid 0 param unexpected type: %T", who)
			}
			addr, err := api.ParseAddress(ab, whoString)
			if err != nil {
				return "", xc.AmountBlockchain{}, fmt.Errorf("TransactionPayment.TransactionFeePaid who invalid address: %v", err)
			}
			amountRaw, ok := ev.GetParam("", 1)
			if !ok {
				return "", xc.AmountBlockchain{}, fmt.Errorf("TransactionPayment.TransactionFeePaid amount missing")
			}
			amount := xc.NewAmountBlockchainFromStr(fmt.Sprint(amountRaw))
			return addr, amount, nil
		}
	}
	return "", xc.AmountBlockchain{}, fmt.Errorf("missing fee")
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
			ExtrinsicHash: hash(&ext),
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
		scans++
		block, err = client.rpc.RPC.Chain.GetBlock(blockHash)
		if err != nil {
			return nil, blockHash, ext, -1, false, err
		}
		// fmt.Println("-- block", block.Block.Header.Number)
		index = -1
		for i, ext := range block.Block.Extrinsics {
			if bytes.Equal(extrinsicHash, hash(&ext)) {
				return block, blockHash, &ext, i, true, nil
			}
		}
		// scan the parent next
		blockHash = block.Block.Header.ParentHash
	}
	// not found
	return nil, blockHash, ext, -1, false, nil
}

func hash(ext *types.Extrinsic) []byte {
	bz, _ := codec.Encode(ext)
	return substratetx.HashSerialized(bz)
}
