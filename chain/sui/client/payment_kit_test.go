package client

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func testKit(t *testing.T, server *rpcServer) func(*xc.ChainConfig) {
	d := paymentKitDeployment(testnetChainID)
	server.object = func(_ context.Context, r *pb.GetObjectRequest) (*pb.GetObjectResponse, error) {
		require.Equal(t, d.RegistryID, r.GetObjectId())
		return &pb.GetObjectResponse{Object: &pb.Object{ObjectId: proto.String(d.RegistryID), ObjectType: proto.String(d.PackageID + "::payment_kit::PaymentRegistry"), Owner: &pb.Owner{Kind: pb.Owner_SHARED.Enum(), Version: proto.Uint64(123)}, Contents: &pb.Bcs{Value: make([]byte, 67)}}}, nil
	}
	return func(c *xc.ChainConfig) { c.SuiPaymentKit = d }
}
func TestPaymentKitFundingMatrix(t *testing.T) {
	for _, token := range []bool{false, true} {
		for _, sponsored := range []bool{false, true} {
			for _, funding := range []string{"address", "coins", "mixed"} {
				for _, gasAddress := range []bool{false, true} {
					name := funding
					if token {
						name += "-token"
					}
					if sponsored {
						name += "-sponsored"
					}
					if gasAddress {
						name += "-addressgas"
					}
					t.Run(name, func(t *testing.T) {
						server := transferServer(t)
						server.balance = func(_ context.Context, r *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
							value := uint64(0)
							isGas := r.GetOwner() == string(sponsor) || token && sameCoin(r.GetCoinType(), sui.NativeCoin)
							if isGas {
								if gasAddress {
									value = 1_000_000_000
								}
							} else if funding != "coins" {
								value = 100_000_000
							}
							return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(value)}}, nil
						}
						server.coins = func(_ context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
							value := uint64(0)
							isGas := r.GetOwner() == string(sponsor) || token && !strings.Contains(r.GetObjectType(), testToken)
							if isGas {
								if !gasAddress {
									value = 1_000_000_000
								}
							} else if funding != "address" {
								value = 100_000_000
							}
							if value == 0 {
								return &pb.ListOwnedObjectsResponse{}, nil
							}
							id := 1
							if isGas {
								id = 3
							}
							return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(id, value)}}, nil
						}
						c := newTestClient(t, server, testKit(t, server))
						opts := []builder.BuilderOption{builder.OptionPublicKey(make([]byte, 32)), builder.OptionTransactionID("42")}
						if token {
							opts = append(opts, builder.OptionContractAddress(testToken))
						}
						if sponsored {
							opts = append(opts, builder.OptionFeePayer(sponsor, make([]byte, 32)))
						}
						value := uint64(10_000_000)
						if funding == "mixed" {
							value = 150_000_000
						}
						args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(value), opts...)
						fetched, err := c.FetchTransferInput(context.Background(), args)
						require.NoError(t, err)
						input := fetched.(*sui.TxInput)
						require.Empty(t, input.TransactionID, "client must not populate trusted ID")
						input.SetTransactionID("42")
						encoded, err := json.Marshal(input)
						require.NoError(t, err)
						var decoded sui.TxInput
						require.NoError(t, json.Unmarshal(encoded, &decoded))
						tb, err := sui.NewTxBuilder(c.Asset.Base())
						require.NoError(t, err)
						tx, err := tb.Transfer(args, &decoded)
						require.NoError(t, err)
						ptb := tx.(*sui.Tx).Tx.Value.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value
						call := ptb.Commands[len(ptb.Commands)-1].(*bcs.Command__MoveCall).Value
						require.Equal(t, bcs.Identifier("process_registry_payment"), call.Function)
						require.Len(t, call.Arguments, 6)
						require.Equal(t, "42", input.TransactionID)
						require.True(t, input.SafeFromDoubleSend(&decoded))
						wire, err := tx.Serialize()
						require.NoError(t, err)
						second, err := tb.Transfer(args, &decoded)
						require.NoError(t, err)
						again, err := second.Serialize()
						require.NoError(t, err)
						require.Equal(t, wire, again)
						args.SetTransactionID("43")
						_, err = tb.Transfer(args, &decoded)
						require.ErrorContains(t, err, "does not match")
					})
				}
			}
		}
	}
}
func TestRegistryRetention(t *testing.T) {
	ttl, err := registryRetention(make([]byte, 67))
	require.NoError(t, err)
	require.EqualValues(t, 30, ttl)
	key := "epoch_expiration_duration"
	configured := append(make([]byte, 64), 1, byte(len(key)))
	configured = append(configured, []byte(key)...)
	configured = append(configured, 0)
	configured = binary.LittleEndian.AppendUint64(configured, 7)
	configured = append(configured, 0, 0)
	ttl, err = registryRetention(configured)
	require.NoError(t, err)
	require.EqualValues(t, 7, ttl)
	configured[len(configured)-3] = 0
	configured[len(configured)-2] = 1
	_, err = registryRetention(configured)
	require.Error(t, err)
	for _, raw := range [][]byte{nil, make([]byte, 66), append(make([]byte, 67), 0)} {
		_, err := registryRetention(raw)
		require.Error(t, err)
	}
	require.Nil(t, paymentKitDeployment("devnet-reset"))
}

func TestPaymentKitApplicationSetterAndUnsupportedNetwork(t *testing.T) {
	server := transferServer(t)
	c := newTestClient(t, server)
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1), builder.OptionPublicKey(make([]byte, 32)), builder.OptionTransactionID("42"))
	_, err := c.FetchTransferInput(context.Background(), args)
	require.ErrorContains(t, err, "no Payment Kit deployment")
	// Metadata is also available when the application only uses the TxInput setter.
	testKit(t, server)(c.Asset)
	plain := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1), builder.OptionPublicKey(make([]byte, 32)))
	input, err := c.FetchTransferInput(context.Background(), plain)
	require.NoError(t, err)
	require.Empty(t, input.(*sui.TxInput).TransactionID)
	tb, err := sui.NewTxBuilder(c.Asset.Base())
	require.NoError(t, err)
	legacy, err := tb.Transfer(plain, input)
	require.NoError(t, err)
	commands := legacy.(*sui.Tx).Tx.Value.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value.Commands
	require.IsType(t, &bcs.Command__TransferObjects{}, commands[len(commands)-1])
	input.(xc.TxInputWithTransactionID).SetTransactionID("42")
	protected, err := tb.Transfer(plain, input)
	require.NoError(t, err)
	commands = protected.(*sui.Tx).Tx.Value.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value.Commands
	require.Equal(t, bcs.Identifier("process_registry_payment"), commands[len(commands)-1].(*bcs.Command__MoveCall).Value.Function)
}

func TestNoIDKeepsCoinsAndBudgetsBothFundingPaths(t *testing.T) {
	server := transferServer(t)
	server.balance = func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
		return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(100_000_000)}}, nil
	}
	server.coins = func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
		return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(1, 10_000_000)}}, nil
	}
	simulations := 0
	server.simulate = func(_ context.Context, r *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
		simulations++
		wire, err := bcs.BcsDeserializeTransactionData(r.Transaction.Bcs.Value)
		require.NoError(t, err)
		data := wire.(*bcs.TransactionData__V1).Value
		fee := uint64(4_000_000)
		if len(data.GasData.Payment) > 0 {
			fee = 6_000_000
		}
		require.GreaterOrEqual(t, data.GasData.Budget, fee)
		effects := successfulEffects()
		effects.GasUsed = &pb.GasCostSummary{ComputationCost: proto.Uint64(fee), StorageCost: proto.Uint64(0), StorageRebate: proto.Uint64(0)}
		return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: effects}}, nil
	}
	c := newTestClient(t, server, testKit(t, server))
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1_000_000), builder.OptionPublicKey(make([]byte, 32)))
	fetched, err := c.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	input := fetched.(*sui.TxInput)
	require.Equal(t, 2, simulations)
	require.Empty(t, input.TransactionID)
	require.NotEmpty(t, input.GasCoin.Digest)
	require.EqualValues(t, 6_000_000, input.GasBudget)
	require.False(t, input.IsFeeLimitAccurate())
	tb, err := sui.NewTxBuilder(c.Asset.Base())
	require.NoError(t, err)
	plain, err := tb.Transfer(args, input)
	require.NoError(t, err)
	require.Len(t, plain.(*sui.Tx).Tx.Value.GasData.Payment, 1)
	require.EqualValues(t, 100_000_000, input.AddressBalance.Balance, "builder must preserve address metadata for the setter")
	input.SetTransactionID("set-after-fetch")
	protected, err := tb.Transfer(args, input)
	require.NoError(t, err)
	require.Empty(t, protected.(*sui.Tx).Tx.Value.GasData.Payment)
	require.False(t, input.IsFeeLimitAccurate(), "setting an ID does not make a two-plan budget exact")
}

func TestAddressOnlyInputCanApplyIDAfterFetch(t *testing.T) {
	server := transferServer(t)
	server.balance = func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
		return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(100_000_000)}}, nil
	}
	server.coins = func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
		return &pb.ListOwnedObjectsResponse{}, nil
	}
	c := newTestClient(t, server, testKit(t, server))
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1_000_000), builder.OptionPublicKey(make([]byte, 32)))
	input, err := c.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	tb, err := sui.NewTxBuilder(c.Asset.Base())
	require.NoError(t, err)
	_, err = tb.Transfer(args, input)
	require.Error(t, err)
	input.(xc.TxInputWithTransactionID).SetTransactionID("trusted")
	_, err = tb.Transfer(args, input)
	require.NoError(t, err)
}

func TestPaymentKitReestimatesAfterFundingChanges(t *testing.T) {
	server := transferServer(t)
	server.balance = func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
		return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(100_000_000)}}, nil
	}
	simulations := 0
	server.simulate = func(_ context.Context, r *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
		simulations++
		wire, err := bcs.BcsDeserializeTransactionData(r.Transaction.Bcs.Value)
		require.NoError(t, err)
		tx := wire.(*bcs.TransactionData__V1).Value
		fee := uint64(2_000_000)
		if len(tx.GasData.Payment) == 0 {
			fee = 6_000_000
		}
		require.GreaterOrEqual(t, tx.GasData.Budget, fee, "re-simulation must have room for its larger fee")
		effects := successfulEffects()
		effects.GasUsed = &pb.GasCostSummary{ComputationCost: proto.Uint64(fee), StorageCost: proto.Uint64(0), StorageRebate: proto.Uint64(0)}
		return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: effects}}, nil
	}
	c := newTestClient(t, server, testKit(t, server))
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1), builder.OptionPublicKey(make([]byte, 32)), builder.OptionTransactionID("reestimate"))
	fetched, err := c.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	input := fetched.(*sui.TxInput)
	require.Equal(t, 2, simulations)
	require.EqualValues(t, 6_000_000, input.GasBudget)
	require.Empty(t, input.GasCoin.Digest)
	require.Empty(t, input.Coins)
	require.Empty(t, input.AddressBalance.GasCoins)
}

func TestPaymentKitKeepsCoinsWhenAddressCannotCoverNewFee(t *testing.T) {
	server := transferServer(t)
	server.balance = func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
		return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(3_000_000)}}, nil
	}
	simulations := 0
	server.simulate = func(_ context.Context, r *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
		simulations++
		wire, err := bcs.BcsDeserializeTransactionData(r.Transaction.Bcs.Value)
		require.NoError(t, err)
		tx := wire.(*bcs.TransactionData__V1).Value
		fee := uint64(2_000_000)
		if len(tx.GasData.Payment) == 0 {
			fee = 6_000_000
		}
		if tx.GasData.Budget < fee {
			return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(false), Error: &pb.ExecutionError{Description: proto.String("InsufficientGas")}}}}}, nil
		}
		effects := successfulEffects()
		effects.GasUsed = &pb.GasCostSummary{ComputationCost: proto.Uint64(fee), StorageCost: proto.Uint64(0), StorageRebate: proto.Uint64(0)}
		return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: effects}}, nil
	}
	c := newTestClient(t, server, testKit(t, server))
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1), builder.OptionPublicKey(make([]byte, 32)), builder.OptionTransactionID("reestimate"))
	fetched, err := c.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	input := fetched.(*sui.TxInput)
	require.Equal(t, 2, simulations)
	require.EqualValues(t, 2_000_000, input.GasBudget)
	require.NotEmpty(t, input.GasCoin.Digest)
	require.NotEmpty(t, input.Coins)
	require.NotEmpty(t, input.AddressBalance.GasCoins)
}

func TestPaymentKitRebuildWithLegacyGasCoinInCoins(t *testing.T) {
	server := transferServer(t)
	c := newTestClient(t, server, testKit(t, server))
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1_000_000), builder.OptionPublicKey(make([]byte, 32)), builder.OptionTransactionID("legacy-coins"))
	fetched, err := c.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	input := fetched.(*sui.TxInput)
	input.AddressBalance = nil
	input.Coins = append([]*types.Coin{&input.GasCoin}, input.Coins...)
	before, err := json.Marshal(input)
	require.NoError(t, err)
	tb, err := sui.NewTxBuilder(c.Asset.Base())
	require.NoError(t, err)
	var first []byte
	for i := 0; i < 3; i++ {
		tx, err := tb.Transfer(args, input)
		require.NoError(t, err)
		wire, err := tx.Serialize()
		require.NoError(t, err)
		if i == 0 {
			first = wire
		} else {
			require.Equal(t, first, wire)
		}
	}
	after, err := json.Marshal(input)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
