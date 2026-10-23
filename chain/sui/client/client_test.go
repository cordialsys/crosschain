package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	xclient "github.com/cordialsys/crosschain/client"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const testDigest = "J2Vkui75vgoLvCmNiREVKwpeTVPCq5EQ71i2ETahP6R9"

var sender = xc.Address("0x" + strings.Repeat("11", 32))
var recipient = xc.Address("0x" + strings.Repeat("22", 32))
var sponsor = xc.Address("0x" + strings.Repeat("33", 32))

const testToken = "0x42::token::TOKEN"

type rpcServer struct {
	pb.UnimplementedStateServiceServer
	pb.UnimplementedLedgerServiceServer
	pb.UnimplementedTransactionExecutionServiceServer
	balance     func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error)
	coins       func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error)
	checkpoint  func(context.Context, *pb.GetCheckpointRequest) (*pb.GetCheckpointResponse, error)
	epoch       func(context.Context, *pb.GetEpochRequest) (*pb.GetEpochResponse, error)
	transaction func(context.Context, *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error)
	execute     func(context.Context, *pb.ExecuteTransactionRequest) (*pb.ExecuteTransactionResponse, error)
	simulate    func(context.Context, *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error)
	object      func(context.Context, *pb.GetObjectRequest) (*pb.GetObjectResponse, error)
	coinInfo    func(context.Context, *pb.GetCoinInfoRequest) (*pb.GetCoinInfoResponse, error)
}

func (s *rpcServer) GetBalance(c context.Context, r *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	if s.balance == nil {
		return &pb.GetBalanceResponse{Balance: &pb.Balance{Balance: proto.Uint64(0)}}, nil
	}
	return s.balance(c, r)
}
func (s *rpcServer) ListOwnedObjects(c context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
	return s.coins(c, r)
}
func (s *rpcServer) GetCheckpoint(c context.Context, r *pb.GetCheckpointRequest) (*pb.GetCheckpointResponse, error) {
	return s.checkpoint(c, r)
}
func (s *rpcServer) GetEpoch(c context.Context, r *pb.GetEpochRequest) (*pb.GetEpochResponse, error) {
	return s.epoch(c, r)
}
func (s *rpcServer) GetTransaction(c context.Context, r *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	return s.transaction(c, r)
}
func (s *rpcServer) ExecuteTransaction(c context.Context, r *pb.ExecuteTransactionRequest) (*pb.ExecuteTransactionResponse, error) {
	if _, err := fieldmaskpb.New(&pb.ExecutedTransaction{}, r.GetReadMask().GetPaths()...); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid execution mask: %v", err)
	}
	return s.execute(c, r)
}
func (s *rpcServer) SimulateTransaction(c context.Context, r *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
	if _, err := fieldmaskpb.New(&pb.SimulateTransactionResponse{}, r.GetReadMask().GetPaths()...); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid simulation mask: %v", err)
	}
	return s.simulate(c, r)
}
func (s *rpcServer) GetObject(c context.Context, r *pb.GetObjectRequest) (*pb.GetObjectResponse, error) {
	return s.object(c, r)
}
func (s *rpcServer) GetCoinInfo(c context.Context, r *pb.GetCoinInfoRequest) (*pb.GetCoinInfoResponse, error) {
	return s.coinInfo(c, r)
}
func (s *rpcServer) GetServiceInfo(context.Context, *pb.GetServiceInfoRequest) (*pb.GetServiceInfoResponse, error) {
	return &pb.GetServiceInfoResponse{CheckpointHeight: proto.Uint64(110), ChainId: proto.String(testDigest)}, nil
}

func newTestClient(t *testing.T, server *rpcServer, configure ...func(*xc.ChainConfig)) *Client {
	t.Helper()
	cfg := xc.NewChainConfig(xc.SUI).WithUrl("http://127.0.0.1:9000").WithDecimals(9)
	for _, fn := range configure {
		fn(cfg)
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterStateServiceServer(grpcServer, server)
	pb.RegisterLedgerServiceServer(grpcServer, server)
	pb.RegisterTransactionExecutionServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() { grpcServer.Stop(); listener.Close() })
	c, err := NewClient(cfg, grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Close()) })
	return c
}
func checkpointResponse(height uint64) *pb.GetCheckpointResponse {
	return &pb.GetCheckpointResponse{Checkpoint: &pb.Checkpoint{SequenceNumber: proto.Uint64(height), Digest: proto.String(testDigest), Summary: &pb.CheckpointSummary{Epoch: proto.Uint64(42), Timestamp: timestamppb.New(time.UnixMilli(1700000000123))}, Contents: &pb.CheckpointContents{Transactions: []*pb.CheckpointedTransactionInfo{{Transaction: proto.String(testDigest)}}}}}
}
func coinObject(id int, balance uint64) *pb.Object {
	return &pb.Object{ObjectId: proto.String(fmt.Sprintf("0x%064x", id)), Version: proto.Uint64(123), Digest: proto.String(testDigest), Balance: proto.Uint64(balance), PreviousTransaction: proto.String(testDigest)}
}
func successfulEffects() *pb.TransactionEffects {
	return &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(true)}, GasUsed: &pb.GasCostSummary{ComputationCost: proto.Uint64(3_000_000), StorageCost: proto.Uint64(2_000_000), StorageRebate: proto.Uint64(1_000_000)}}
}
func transferServer(t *testing.T) *rpcServer {
	return &rpcServer{
		coins: func(ctx context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
			require.Contains(t, r.ReadMask.Paths, "digest")
			require.Contains(t, r.ReadMask.Paths, "balance")
			if r.GetOwner() == string(sponsor) {
				return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(3, 10_000_000_000)}}, nil
			}
			if strings.Contains(r.GetObjectType(), testToken) {
				return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(4, 1_000_000)}}, nil
			}
			return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(1, 10_000_000_000), coinObject(2, 5_000_000_000)}}, nil
		},
		checkpoint: func(_ context.Context, r *pb.GetCheckpointRequest) (*pb.GetCheckpointResponse, error) {
			return checkpointResponse(100), nil
		},
		epoch: func(_ context.Context, r *pb.GetEpochRequest) (*pb.GetEpochResponse, error) {
			require.Contains(t, r.ReadMask.Paths, "reference_gas_price")
			return &pb.GetEpochResponse{Epoch: &pb.Epoch{ReferenceGasPrice: proto.Uint64(1000)}}, nil
		},
		simulate: func(_ context.Context, r *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
			require.Equal(t, []string{"transaction.effects.status", "transaction.effects.gas_used"}, r.ReadMask.Paths)
			// Decode with the pre-existing builder's BCS schema.
			_, err := bcs.BcsDeserializeTransactionData(r.GetTransaction().GetBcs().GetValue())
			require.NoError(t, err)
			return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: successfulEffects()}}, nil
		},
	}
}
func TestBalanceAndMetadata(t *testing.T) {
	server := &rpcServer{}
	server.balance = func(ctx context.Context, r *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
		require.Equal(t, string(sender), r.GetOwner())
		require.Equal(t, sui.NativeCoin, r.GetCoinType())
		md, _ := metadata.FromIncomingContext(ctx)
		require.Equal(t, []string{"Bearer test-token"}, md.Get("authorization"))
		_, ok := ctx.Deadline()
		require.True(t, ok)
		return &pb.GetBalanceResponse{Balance: &pb.Balance{Balance: proto.Uint64(30), CoinBalance: proto.Uint64(10), AddressBalance: proto.Uint64(20)}}, nil
	}
	server.coinInfo = func(_ context.Context, r *pb.GetCoinInfoRequest) (*pb.GetCoinInfoResponse, error) {
		require.Equal(t, testToken, r.GetCoinType())
		return &pb.GetCoinInfoResponse{Metadata: &pb.CoinMetadata{Decimals: proto.Uint32(6)}}, nil
	}
	c := newTestClient(t, server, func(cfg *xc.ChainConfig) { cfg.Auth2 = "raw:test-token" })
	amount, err := c.FetchBalance(context.Background(), xclient.NewBalanceArgs(sender))
	require.NoError(t, err)
	require.Equal(t, "30", amount.String())
	decimals, err := c.FetchDecimals(context.Background(), testToken)
	require.NoError(t, err)
	require.Equal(t, 6, decimals)
	decimals, err = c.FetchDecimals(context.Background(), xc.ContractAddress("0x"+strings.Repeat("0", 63)+"2::sui::SUI"))
	require.NoError(t, err)
	require.Equal(t, 9, decimals)
}
func TestCoinPaginationAndEncoding(t *testing.T) {
	calls := 0
	server := &rpcServer{coins: func(_ context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
		calls++
		require.Equal(t, "0x2::coin::Coin<"+testToken+">", r.GetObjectType())
		if calls == 1 {
			require.Empty(t, r.PageToken)
			return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(1, 9_007_199_254_740_993)}, NextPageToken: []byte{1, 2, 3}}, nil
		}
		require.Equal(t, []byte{1, 2, 3}, r.PageToken)
		return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(2, 7)}}, nil
	}}
	c := newTestClient(t, server)
	coins, err := c.GetAllCoinsFor(context.Background(), sender, testToken)
	require.NoError(t, err)
	require.Len(t, coins, 2)
	require.Equal(t, uint64(9_007_199_254_740_993), coins[0].Balance.Uint64())
	encoded, err := json.Marshal(coins[0])
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"coinType":%q,"coinObjectId":"0x%064x","version":"123","digest":%q,"balance":"9007199254740993","previousTransaction":%q}`, testToken, 1, testDigest, testDigest), string(encoded))
}
func TestTransferInputCompatibleWithExistingBuilder(t *testing.T) {
	for _, tc := range []struct {
		name             string
		token, sponsored bool
	}{{"native", false, false}, {"token", true, false}, {"sponsored_native", false, true}, {"sponsored_token", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, transferServer(t))
			opts := []builder.BuilderOption{builder.OptionPublicKey(make([]byte, 32))}
			if tc.token {
				opts = append(opts, builder.OptionContractAddress(testToken))
			}
			if tc.sponsored {
				opts = append(opts, builder.OptionFeePayer(sponsor, make([]byte, 32)))
			}
			args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(100), opts...)
			input, err := c.FetchTransferInput(context.Background(), args)
			require.NoError(t, err)
			legacy, ok := input.(*sui.TxInput)
			require.True(t, ok)
			require.Equal(t, uint64(42), legacy.CurrentEpoch)
			require.Equal(t, uint64(1000), legacy.GasPrice)
			expectedBudget := uint64(4_000_000)
			if tc.token {
				expectedBudget = 4_400_000
			}
			require.Equal(t, expectedBudget, legacy.GasBudget)
			owner := sender
			if tc.sponsored {
				owner = sponsor
			}
			require.Equal(t, owner, legacy.GasCoinOwner)
			data, err := json.Marshal(input)
			require.NoError(t, err)
			// This is precisely what an older offline builder receives.
			var decoded sui.TxInput
			require.NoError(t, json.Unmarshal(data, &decoded))
			require.Equal(t, *legacy, decoded)
			builder, err := sui.NewTxBuilder(c.Asset.Base())
			require.NoError(t, err)
			tx, err := builder.Transfer(args, legacy)
			require.NoError(t, err)
			roundTripped, err := builder.Transfer(args, &decoded)
			require.NoError(t, err)
			first, err := tx.Serialize()
			require.NoError(t, err)
			second, err := roundTripped.Serialize()
			require.NoError(t, err)
			require.Equal(t, first, second)
			hashes, err := tx.Sighashes()
			require.NoError(t, err)
			count := 1
			if tc.sponsored {
				count = 2
			}
			require.Len(t, hashes, count)
		})
	}
}
func TestSubmitPreservesExistingSignedPayload(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			data := []byte{1, 2, 3}
			signatures := [][]byte{make([]byte, 97), append([]byte{0}, make([]byte, 96)...)}
			signatures[1][1] = 7
			server := &rpcServer{execute: func(_ context.Context, r *pb.ExecuteTransactionRequest) (*pb.ExecuteTransactionResponse, error) {
				require.Equal(t, []string{"effects.status"}, r.ReadMask.Paths)
				require.Equal(t, data, r.GetTransaction().GetBcs().GetValue())
				require.Len(t, r.Signatures, 2)
				for i, sig := range signatures {
					require.Equal(t, sig, r.Signatures[i].GetBcs().GetValue())
				}
				return &pb.ExecuteTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: successfulEffects()}}, nil
			}}
			c := newTestClient(t, server)
			req := xctypes.SubmitTxReq{TxData: data}
			if legacy {
				req.LegacyTxSignatures = signatures
			} else {
				encoded, err := json.Marshal(sui.BroadcastMetadata{Signatures: signatures})
				require.NoError(t, err)
				req.BroadcastInput = string(encoded)
			}
			require.NoError(t, c.SubmitTx(context.Background(), req, builder.SubmitArgs{}))
		})
	}
}
func TestCheckpointAndTransactionInfo(t *testing.T) {
	server := &rpcServer{checkpoint: func(_ context.Context, r *pb.GetCheckpointRequest) (*pb.GetCheckpointResponse, error) {
		require.Contains(t, r.ReadMask.Paths, "contents.transactions.transaction")
		height := uint64(100)
		if r.CheckpointId != nil {
			height = r.GetSequenceNumber()
		}
		return checkpointResponse(height), nil
	}, transaction: func(_ context.Context, r *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
		require.Contains(t, r.ReadMask.Paths, "balance_changes")
		return &pb.GetTransactionResponse{Transaction: &pb.ExecutedTransaction{Digest: proto.String(testDigest), Checkpoint: proto.Uint64(100), Timestamp: timestamppb.New(time.Unix(1700000000, 0)), Effects: successfulEffects(), BalanceChanges: []*pb.BalanceChange{
			{Address: proto.String(string(sender)), CoinType: proto.String("0x" + strings.Repeat("0", 63) + "2::sui::SUI"), Amount: proto.String("-110")},
			{Address: proto.String(string(recipient)), CoinType: proto.String(sui.NativeCoin), Amount: proto.String("100")},
		}}}, nil
	}}
	c := newTestClient(t, server)
	block, err := c.FetchBlock(context.Background(), xclient.AtHeight(0))
	require.NoError(t, err)
	require.Equal(t, []string{testDigest}, block.TransactionIds)
	info, err := c.FetchLegacyTxInfo(context.Background(), testDigest)
	require.NoError(t, err)
	require.Equal(t, int64(10), info.Confirmations)
	require.Equal(t, int64(1700000000), info.BlockTime)
	require.Equal(t, "10", info.Fee.String())
	require.Equal(t, sender, info.From)
	require.Equal(t, recipient, info.To)
	require.Equal(t, xc.TxStatusSuccess, info.Status)
	require.Empty(t, info.ContractAddress)
}
func TestMissingTransaction(t *testing.T) {
	c := newTestClient(t, &rpcServer{transaction: func(context.Context, *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
		return nil, status.Error(codes.NotFound, "pruned or missing")
	}})
	_, err := c.FetchLegacyTxInfo(context.Background(), testDigest)
	require.Error(t, err)
	var typed *xcerrors.Error
	require.ErrorAs(t, err, &typed)
	require.Equal(t, xcerrors.TransactionNotFound, typed.Status)
}
func TestEndpoint(t *testing.T) {
	for _, tc := range []struct {
		url, target string
		secure      bool
	}{{"https://fullnode.mainnet.sui.io:443/", "dns:///fullnode.mainnet.sui.io:443", true}, {"fullnode.mainnet.sui.io", "dns:///fullnode.mainnet.sui.io:443", true}, {"http://localhost:9000", "dns:///localhost:9000", false}, {"http://[::1]:9000", "dns:///[::1]:9000", false}} {
		endpoint, err := parseEndpoint(tc.url)
		require.NoError(t, err)
		require.Equal(t, tc.target, endpoint.target)
		require.Equal(t, tc.secure, endpoint.secure)
	}
	for _, raw := range []string{"", "ftp://host", "https://host/#fragment"} {
		_, err := parseEndpoint(raw)
		require.Error(t, err)
	}
}
func TestStakingKeepsOnlyDiscoveryOnJSONRPC(t *testing.T) {
	calls := 0
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			ID     any    `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "suix_getStakes", request.Method)
		calls++
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": []any{map[string]any{"validatorAddress": recipient, "stakingPool": sponsor, "stakes": []any{map[string]any{"stakedSuiId": fmt.Sprintf("0x%064x", 9), "stakeRequestEpoch": "40", "stakeActiveEpoch": "41", "principal": "2000000000", "status": "Pending"}}}}}))
	}))
	defer httpServer.Close()
	server := transferServer(t)
	server.object = func(_ context.Context, r *pb.GetObjectRequest) (*pb.GetObjectResponse, error) {
		require.Equal(t, fmt.Sprintf("0x%064x", 9), r.GetObjectId())
		return &pb.GetObjectResponse{Object: coinObject(9, 2_000_000_000)}, nil
	}
	c := newTestClient(t, server, func(cfg *xc.ChainConfig) { cfg.SecondaryURL = httpServer.URL })
	args := buildertest.MustNewStakingArgs(xc.SUI, sender, builder.OptionPublicKey(make([]byte, 32)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(2_000_000_000)))
	input, err := c.FetchUnstakingInput(context.Background(), args)
	require.NoError(t, err)
	require.IsType(t, &sui.UnstakingInput{}, input)
	require.Equal(t, 1, calls)
	require.Equal(t, uint64(123), input.(*sui.UnstakingInput).StakesToUnstake[0].Version)
	stakeArgs := buildertest.MustNewStakingArgs(xc.SUI, sender, builder.OptionValidator(string(recipient)), builder.OptionPublicKey(make([]byte, 32)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(2_000_000_000)))
	stake, err := c.FetchStakingInput(context.Background(), stakeArgs)
	require.NoError(t, err)
	require.IsType(t, &sui.StakingInput{}, stake)
	require.Equal(t, 1, calls) // Staking itself does not require the old API.
}

func TestGasSimulationValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		effects   *pb.TransactionEffects
		budget    uint64
		wantError bool
	}{
		{"normal", successfulEffects(), 4_000_000, false},
		{"rebate_exceeds_cost", &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(true)}, GasUsed: &pb.GasCostSummary{ComputationCost: proto.Uint64(1), StorageCost: proto.Uint64(2), StorageRebate: proto.Uint64(10)}}, 2_000_000, false},
		{"failed_execution", &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(false)}}, 0, true},
		{"large_rebate_cannot_fund_computation", &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(true)}, GasUsed: &pb.GasCostSummary{ComputationCost: proto.Uint64(10_000_000), StorageCost: proto.Uint64(1_000_000), StorageRebate: proto.Uint64(20_000_000)}}, 10_000_000, false},
		{"missing_effects", nil, 0, true},
		{"missing_gas", &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(true)}}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, &rpcServer{simulate: func(context.Context, *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
				return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: tc.effects}}, nil
			}})
			budget, err := c.simulateTransactionGasFee(context.Background(), &xctypes.SubmitTxReq{TxData: []byte{1}}, true)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.budget, budget)
		})
	}
	server := transferServer(t)
	server.epoch = func(context.Context, *pb.GetEpochRequest) (*pb.GetEpochResponse, error) {
		return nil, status.Error(codes.Unavailable, "unavailable")
	}
	c := newTestClient(t, server, func(cfg *xc.ChainConfig) { cfg.ChainGasPriceDefault = 1234 })
	input, err := c.fetchBaseInput(context.Background(), sui.NativeCoin, sender, "")
	require.NoError(t, err)
	require.Equal(t, uint64(1234), input.GasPrice)
}

func TestMalformedResponsesAndExecutionFailure(t *testing.T) {
	t.Run("missing_balance", func(t *testing.T) {
		c := newTestClient(t, &rpcServer{balance: func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
			return &pb.GetBalanceResponse{}, nil
		}})
		_, err := c.FetchBalance(context.Background(), xclient.NewBalanceArgs(sender))
		require.ErrorContains(t, err, "no balance")
	})
	t.Run("incomplete_coin", func(t *testing.T) {
		c := newTestClient(t, &rpcServer{coins: func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
			object := coinObject(1, 0)
			object.Digest = nil
			return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{object}}, nil
		}})
		_, err := c.GetAllCoinsFor(context.Background(), sender, sui.NativeCoin)
		require.ErrorContains(t, err, "incomplete coin")
	})
	t.Run("repeated_page", func(t *testing.T) {
		c := newTestClient(t, &rpcServer{coins: func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
			return &pb.ListOwnedObjectsResponse{NextPageToken: []byte{1}}, nil
		}})
		_, err := c.GetAllCoinsFor(context.Background(), sender, sui.NativeCoin)
		require.ErrorContains(t, err, "repeated coin page")
	})
	t.Run("failed_submission", func(t *testing.T) {
		c := newTestClient(t, &rpcServer{execute: func(context.Context, *pb.ExecuteTransactionRequest) (*pb.ExecuteTransactionResponse, error) {
			return &pb.ExecuteTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(false), Error: &pb.ExecutionError{Description: proto.String("MoveAbort")}}}}}, nil
		}})
		err := c.SubmitTx(context.Background(), xctypes.SubmitTxReq{TxData: []byte{1}, LegacyTxSignatures: [][]byte{{0, 1}}}, builder.SubmitArgs{})
		require.ErrorContains(t, err, "MoveAbort")
	})
	t.Run("invalid_metadata", func(t *testing.T) {
		c := newTestClient(t, &rpcServer{})
		err := c.SubmitTx(context.Background(), xctypes.SubmitTxReq{BroadcastInput: "{"}, builder.SubmitArgs{})
		require.ErrorContains(t, err, "broadcast metadata")
	})
	t.Run("deadline", func(t *testing.T) {
		c := newTestClient(t, &rpcServer{balance: func(ctx context.Context, _ *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}, func(cfg *xc.ChainConfig) { cfg.HttpTimeout = 100 * time.Millisecond })
		_, err := c.FetchBalance(context.Background(), xclient.NewBalanceArgs(sender))
		require.Equal(t, codes.DeadlineExceeded, status.Code(err))
	})
}

func TestNoGasFunding(t *testing.T) {
	c := newTestClient(t, &rpcServer{coins: func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
		return &pb.ListOwnedObjectsResponse{}, nil
	}})
	args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(1))
	_, err := c.FetchTransferInput(context.Background(), args)
	require.ErrorContains(t, err, "no SUI coin objects or address balance available for gas")
}
