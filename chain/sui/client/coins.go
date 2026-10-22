package client

import (
	"bytes"
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"github.com/cordialsys/go-sui-sdk/v2/lib"
	"github.com/cordialsys/go-sui-sdk/v2/move_types"
	"github.com/cordialsys/go-sui-sdk/v2/sui_types"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"google.golang.org/protobuf/proto"
)

// GetAllCoinsFor returns the same object-backed coins used by older builders.
// Address balances are intentionally not synthesized into coin objects.
func (c *Client) GetAllCoinsFor(ctx context.Context, address xc.Address, coinType string) ([]*types.Coin, error) {
	owner, err := move_types.NewAccountAddressHex(string(address))
	if err != nil {
		return nil, fmt.Errorf("invalid Sui address: %w", err)
	}
	coinType = sui.NormalizeCoinContract(coinType)
	req := &pb.ListOwnedObjectsRequest{
		Owner:      proto.String(owner.String()),
		ObjectType: proto.String("0x2::coin::Coin<" + coinType + ">"),
		PageSize:   proto.Uint32(250),
		ReadMask:   mask("object_id", "version", "digest", "balance", "previous_transaction"),
	}
	coins := []*types.Coin{}
	for {
		resp, err := c.State.ListOwnedObjects(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("Sui returned no owned objects response")
		}
		for _, object := range resp.Objects {
			coin, err := objectCoin(object, coinType)
			if err != nil {
				return nil, err
			}
			coins = append(coins, coin)
		}
		if len(resp.NextPageToken) == 0 {
			return coins, nil
		}
		if bytes.Equal(req.PageToken, resp.NextPageToken) {
			return nil, fmt.Errorf("Sui returned a repeated coin page token")
		}
		req.PageToken = resp.NextPageToken
	}
}

func objectCoin(object *pb.Object, coinType string) (*types.Coin, error) {
	if object == nil || object.ObjectId == nil || object.Version == nil || object.Digest == nil || object.Balance == nil {
		return nil, fmt.Errorf("Sui returned an incomplete coin object")
	}
	id, err := move_types.NewAccountAddressHex(object.GetObjectId())
	if err != nil {
		return nil, fmt.Errorf("invalid coin object ID: %w", err)
	}
	digest, err := lib.NewBase58(object.GetDigest())
	if err != nil || digest == nil || len(*digest) != 32 {
		return nil, fmt.Errorf("invalid coin object digest %q", object.GetDigest())
	}
	coin := &types.Coin{
		CoinType:     coinType,
		CoinObjectId: *id,
		Version:      types.NewSafeSuiBigInt(object.GetVersion()),
		Digest:       sui_types.ObjectDigest(*digest),
		Balance:      types.NewSafeSuiBigInt(object.GetBalance()),
	}
	if previous := object.GetPreviousTransaction(); previous != "" {
		digest, err := lib.NewBase58(previous)
		if err != nil || digest == nil || len(*digest) != 32 {
			return nil, fmt.Errorf("invalid previous transaction digest %q", previous)
		}
		coin.PreviousTransaction = sui_types.TransactionDigest(*digest)
	}
	return coin, nil
}

func (c *Client) EstimateGas(ctx context.Context) (xc.AmountBlockchain, error) {
	resp, err := c.Ledger.GetEpoch(ctx, &pb.GetEpochRequest{ReadMask: mask("reference_gas_price")})
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	if resp.GetEpoch() == nil || resp.Epoch.ReferenceGasPrice == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("Sui returned no reference gas price")
	}
	return xc.NewAmountBlockchainFromUint64(resp.Epoch.GetReferenceGasPrice()), nil
}
