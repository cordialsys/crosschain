package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func queueReproResources(t *testing.T, tx xc.Tx) map[string]bool {
	t.Helper()
	raw, err := tx.Serialize()
	require.NoError(t, err)
	data, err := bcs.BcsDeserializeTransactionData(raw)
	require.NoError(t, err)
	v := data.(*bcs.TransactionData__V1).Value
	refs := map[string]bool{}
	for _, gas := range v.GasData.Payment {
		refs[fmt.Sprintf("%x/%d", gas.Field0.Value, gas.Field1)] = true
	}
	for _, arg := range v.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value.Inputs {
		if object, ok := arg.(*bcs.CallArg__Object); ok {
			if owned, ok := object.Value.(*bcs.ObjectArg__ImmOrOwnedObject); ok {
				refs[fmt.Sprintf("%x/%d", owned.Field0.Value, owned.Field1)] = true
			}
		}
	}
	return refs
}

func TestFeePayerQueueAfterSuccessfulTransfer(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		senderAddress                            uint64
		addressGas, switchToCoins, legacy, token bool
	}{
		{name: "coin_sender_address_sponsor", addressGas: true},
		{name: "mixed_sender_address_sponsor", senderAddress: 10_000_000, addressGas: true},
		{name: "address_to_coin_sponsor", addressGas: true, switchToCoins: true},
		{name: "token_sender_address_sponsor", addressGas: true, token: true},
		{name: "mixed_token_sender_address_sponsor", senderAddress: 10_000_000, addressGas: true, token: true},
		{name: "coin_sender_coin_sponsor"},
		{name: "legacy_coin_sponsor", legacy: true, addressGas: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var executed atomic.Bool
			server := transferServer(t)
			server.coins = func(_ context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
				id, value := 1, uint64(20_000_000)
				if tc.senderAddress > 0 {
					value = 10_000_000
				}
				sponsorCoin := r.GetOwner() == string(sponsor)
				if sponsorCoin {
					id, value = 3, 10_000_000
				}
				coin := coinObject(id, value)
				if executed.Load() && (!sponsorCoin || !tc.addressGas || tc.legacy) {
					coin.Version = proto.Uint64(124)
				}
				return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coin}}, nil
			}
			server.balance = func(_ context.Context, r *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
				value := tc.senderAddress
				if r.GetOwner() == string(sponsor) {
					value = 0
					if tc.addressGas {
						value = 100_000_000
					}
					if tc.switchToCoins && executed.Load() {
						value = 5_000_000
					}
				}
				return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(value)}}, nil
			}
			c := newTestClient(t, server, testKit(t, server))
			tb, err := sui.NewTxBuilder(c.Asset.Base())
			require.NoError(t, err)
			fetch := func(id string) (*sui.TxInput, xc.Tx) {
				opts := []builder.BuilderOption{builder.OptionPublicKey(make([]byte, 32)), builder.OptionFeePayer(sponsor, make([]byte, 32))}
				if tc.token {
					opts = append(opts, builder.OptionContractAddress(testToken))
				}
				if !tc.legacy {
					opts = append(opts, builder.OptionTransactionID(id))
				}
				args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(15_000_000), opts...)
				in, err := c.FetchTransferInput(context.Background(), args)
				require.NoError(t, err)
				if !tc.legacy {
					in.(xc.TxInputWithTransactionID).SetTransactionID(id)
				}
				// Include the cached history serialization boundary.
				raw, err := json.Marshal(in)
				require.NoError(t, err)
				var input sui.TxInput
				require.NoError(t, json.Unmarshal(raw, &input))
				tx, err := tb.Transfer(args, &input)
				require.NoError(t, err)
				return &input, tx
			}
			old, oldTx := fetch("first")
			pending, _ := fetch("second")
			require.False(t, pending.IndependentOf(old), "the sender's unspent coin still conflicts before execution")
			executed.Store(true)
			next, nextTx := fetch("second")
			require.Equal(t, old.CurrentEpoch, next.CurrentEpoch)
			before, after := queueReproResources(t, oldTx), queueReproResources(t, nextTx)
			for ref := range before {
				require.False(t, after[ref], "actual transaction object references must be disjoint")
			}
			if tc.addressGas && !tc.legacy {
				require.True(t, sui.CoinEqual(&old.GasCoin, &next.GasCoin), "unused sponsor coin must retain the same version")
				require.Empty(t, oldTx.(*sui.Tx).Tx.Value.GasData.Payment)
			}
			require.False(t, next.SafeFromDoubleSend(old), "distinct IDs and disjoint consumed objects are not retry protection")
			if !tc.legacy {
				same := next.Clone()
				same.SetTransactionID("first")
				require.False(t, same.IndependentOf(old), "same ID must remain blocked")
			}
			require.True(t, next.IndependentOf(old), "a fresh input must not stall on sponsor coins the previous transaction never used")
		})
	}
}
