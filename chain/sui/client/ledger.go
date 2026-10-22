package client

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/cordialsys/go-sui-sdk/v2/lib"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (c *Client) getObject(ctx context.Context, id string) (*pb.Object, error) {
	resp, err := c.Ledger.GetObject(ctx, &pb.GetObjectRequest{ObjectId: proto.String(id), ReadMask: mask("object_id", "version", "digest")})
	if err != nil {
		return nil, err
	}
	obj := resp.GetObject()
	if obj == nil || obj.Version == nil || obj.Digest == nil {
		return nil, fmt.Errorf("Sui returned an incomplete object reference")
	}
	return obj, nil
}

func (c *Client) FetchLatestCheckpoint(ctx context.Context) (*sui.Checkpoint, error) {
	return c.checkpoint(ctx, &pb.GetCheckpointRequest{})
}

func (c *Client) FetchCheckpoint(ctx context.Context, height uint64) (*sui.Checkpoint, error) {
	return c.checkpoint(ctx, &pb.GetCheckpointRequest{CheckpointId: &pb.GetCheckpointRequest_SequenceNumber{SequenceNumber: height}})
}

func (c *Client) checkpoint(ctx context.Context, req *pb.GetCheckpointRequest) (*sui.Checkpoint, error) {
	req.ReadMask = mask(
		"sequence_number", "digest", "summary.epoch", "summary.timestamp",
		"summary.previous_digest", "summary.total_network_transactions", "contents.transactions.transaction",
	)
	resp, err := c.Ledger.GetCheckpoint(ctx, req)
	if err != nil {
		return nil, err
	}
	cp := resp.GetCheckpoint()
	if cp == nil || cp.SequenceNumber == nil || cp.Digest == nil || cp.GetSummary().GetTimestamp() == nil || cp.Summary.Epoch == nil {
		return nil, fmt.Errorf("Sui returned an incomplete checkpoint")
	}
	if err := cp.Summary.Timestamp.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid checkpoint timestamp: %w", err)
	}
	result := &sui.Checkpoint{
		SequenceNumber:           strconv.FormatUint(cp.GetSequenceNumber(), 10),
		Digest:                   cp.GetDigest(),
		Epoch:                    strconv.FormatUint(cp.Summary.GetEpoch(), 10),
		TimestampMs:              strconv.FormatInt(cp.Summary.Timestamp.AsTime().UnixMilli(), 10),
		PreviousDigest:           cp.Summary.GetPreviousDigest(),
		NetworkTotalTransactions: strconv.FormatUint(cp.Summary.GetTotalNetworkTransactions(), 10),
	}
	for _, tx := range cp.GetContents().GetTransactions() {
		result.Transactions = append(result.Transactions, tx.GetTransaction())
	}
	return result, nil
}

func (c *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	var cp *sui.Checkpoint
	var err error
	if height, ok := args.Height(); ok {
		cp, err = c.FetchCheckpoint(ctx, height)
	} else {
		cp, err = c.FetchLatestCheckpoint(ctx)
	}
	if err != nil {
		return nil, err
	}
	timestamp, err := strconv.ParseInt(cp.TimestampMs, 10, 64)
	if err != nil {
		return nil, err
	}
	return &txinfo.BlockWithTransactions{Block: *txinfo.NewBlock(c.Asset.Chain, cp.GetSequenceNumber(), cp.Digest, time.UnixMilli(timestamp)), TransactionIds: cp.Transactions}, nil
}

func (c *Client) FetchLegacyTxInfo(ctx context.Context, hash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	result := txinfo.LegacyTxInfo{}
	digest, err := lib.NewBase58(string(hash))
	if err != nil || digest == nil || len(*digest) != 32 {
		return result, fmt.Errorf("invalid Sui transaction digest")
	}
	resp, err := c.Ledger.GetTransaction(ctx, &pb.GetTransactionRequest{
		Digest:   proto.String(string(hash)),
		ReadMask: mask("digest", "checkpoint", "timestamp", "balance_changes", "effects.status"),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return result, errors.TransactionNotFoundf("%v", err)
		}
		return result, err
	}
	tx := resp.GetTransaction()
	if tx == nil || tx.Digest == nil || tx.Checkpoint == nil || tx.Timestamp == nil || tx.GetEffects().GetStatus() == nil || tx.Effects.Status.Success == nil {
		return result, fmt.Errorf("Sui returned incomplete transaction information")
	}
	if err := tx.Timestamp.CheckValid(); err != nil {
		return result, err
	}
	cp, err := c.FetchCheckpoint(ctx, tx.GetCheckpoint())
	if err != nil {
		return result, err
	}
	latest, err := c.Ledger.GetServiceInfo(ctx, &pb.GetServiceInfoRequest{})
	if err != nil {
		return result, err
	}
	if latest == nil || latest.CheckpointHeight == nil {
		return result, fmt.Errorf("Sui returned no checkpoint height")
	}
	result.BlockHash = cp.Digest
	result.BlockIndex = int64(tx.GetCheckpoint())
	result.BlockTime = tx.Timestamp.AsTime().Unix()
	result.TxID = tx.GetDigest()
	result.Confirmations = int64(latest.GetCheckpointHeight()) - result.BlockIndex
	if result.Confirmations < 0 {
		result.Confirmations = 0
	}
	result.Status = xc.TxStatusSuccess
	if !tx.Effects.Status.GetSuccess() {
		result.Status = xc.TxStatusFailure
		result.Error = executionError(tx.Effects.Status)
	}
	sent, received := xc.NewAmountBlockchainFromUint64(0), xc.NewAmountBlockchainFromUint64(0)
	for i, change := range tx.BalanceChanges {
		if change == nil || change.Address == nil || change.CoinType == nil || change.Amount == nil {
			return txinfo.LegacyTxInfo{}, fmt.Errorf("Sui returned an incomplete balance change")
		}
		integer, ok := new(big.Int).SetString(change.GetAmount(), 10)
		if !ok {
			return txinfo.LegacyTxInfo{}, fmt.Errorf("invalid Sui balance change %q", change.GetAmount())
		}
		amount := xc.AmountBlockchain(*integer)
		native := sameCoin(change.GetCoinType(), c.nativeCoin())
		contract := xc.ContractAddress(change.GetCoinType())
		asset := ""
		if native {
			contract = ""
			asset = string(c.Asset.Chain)
		}
		endpoint := &txinfo.LegacyTxInfoEndpoint{
			Asset:           asset,
			ContractAddress: contract,
			Amount:          amount.Abs(),
			Address:         xc.Address(change.GetAddress()),
			NativeAsset:     c.Asset.Chain,
			Event:           txinfo.NewEventFromIndex(uint64(i), txinfo.MovementVariantNative),
		}
		result.ContractAddress = contract
		if amount.Sign() < 0 {
			result.From = endpoint.Address
			result.Sources = append(result.Sources, endpoint)
			if native {
				sent = sent.Add(&endpoint.Amount)
			}
		} else {
			result.To = endpoint.Address
			result.Amount = amount
			result.Destinations = append(result.Destinations, endpoint)
			if native {
				received = received.Add(&amount)
			}
		}
	}
	result.Fee = sent.Sub(&received)
	return result, nil
}

func (c *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	legacy, err := c.FetchLegacyTxInfo(ctx, args.TxHash())
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	// Balance changes already account for fees, as in the original Sui client.
	legacy.Fee = xc.NewAmountBlockchainFromUint64(0)
	return txinfo.TxInfoFromLegacy(c.Asset, legacy, txinfo.Utxo), nil
}
