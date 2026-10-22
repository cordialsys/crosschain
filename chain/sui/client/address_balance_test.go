package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func TestAddressBalanceTransfers(t *testing.T) {
	for _, tc := range []struct {
		name                                                     string
		token, sponsored                                         bool
		coins, address, gasCoins, gasAddress, amount, withdrawal uint64
		addressGas                                               bool
	}{
		{name: "address_only", address: 100_000_000, amount: 10_000_000, withdrawal: 10_000_000, addressGas: true},
		{name: "mixed_address_gas", coins: 10_000_000, address: 30_000_000, amount: 35_000_000, withdrawal: 25_000_000, addressGas: true},
		{name: "mixed_coin_gas", coins: 30_000_000, address: 10_000_000, amount: 35_000_000, withdrawal: 9_000_000},
		{name: "coins_with_address_gas", coins: 10_000_000, address: 30_000_000, amount: 5_000_000, addressGas: true},
		{name: "token_address_only", token: true, address: 100, gasAddress: 100_000_000, amount: 50, withdrawal: 50, addressGas: true},
		{name: "token_mixed_coin_gas", token: true, coins: 40, address: 60, gasCoins: 100_000_000, amount: 70, withdrawal: 30},
		{name: "token_mixed_address_gas", token: true, coins: 40, address: 60, gasAddress: 100_000_000, amount: 70, withdrawal: 30, addressGas: true},
		{name: "sponsored_address", sponsored: true, address: 100_000_000, gasAddress: 100_000_000, amount: 99_000_000, withdrawal: 99_000_000, addressGas: true},
		{name: "sponsored_coin_gas", sponsored: true, coins: 20_000_000, address: 20_000_000, gasCoins: 100_000_000, amount: 35_000_000, withdrawal: 15_000_000},
		{name: "sponsored_token", sponsored: true, token: true, coins: 40, address: 60, gasAddress: 100_000_000, amount: 70, withdrawal: 30, addressGas: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := transferServer(t)
			server.balance = func(_ context.Context, r *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
				balance := tc.address
				if r.GetOwner() == string(sponsor) || tc.token && sameCoin(r.GetCoinType(), sui.NativeCoin) {
					balance = tc.gasAddress
				}
				return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(balance)}}, nil
			}
			server.coins = func(_ context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
				balance := tc.coins
				id := 1
				if r.GetOwner() == string(sponsor) || tc.token && !strings.Contains(r.GetObjectType(), testToken) {
					balance = tc.gasCoins
					id = 3
				}
				if balance == 0 {
					return &pb.ListOwnedObjectsResponse{}, nil
				}
				// Exercise gas smashing and merging multiple objects, not just one coin.
				return &pb.ListOwnedObjectsResponse{Objects: []*pb.Object{coinObject(id, balance/2), coinObject(id+1, balance-balance/2)}}, nil
			}
			c := newTestClient(t, server)
			options := []builder.BuilderOption{builder.OptionPublicKey(make([]byte, 32))}
			if tc.token {
				options = append(options, builder.OptionContractAddress(testToken))
			}
			if tc.sponsored {
				options = append(options, builder.OptionFeePayer(sponsor, make([]byte, 32)))
			}
			args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(tc.amount), options...)
			fetched, err := c.FetchTransferInput(context.Background(), args)
			require.NoError(t, err)
			input := fetched.(*sui.TxInput)
			require.NotNil(t, input.AddressBalance)
			require.Equal(t, tc.coins+tc.address, input.TotalBalance().Uint64())
			encoded, err := json.Marshal(input)
			require.NoError(t, err)
			var decoded sui.TxInput
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			txBuilder, err := sui.NewTxBuilder(c.Asset.Base())
			require.NoError(t, err)
			tx, err := txBuilder.Transfer(args, &decoded)
			require.NoError(t, err)
			wire, err := tx.Serialize()
			require.NoError(t, err)
			data, err := bcs.BcsDeserializeTransactionData(wire)
			require.NoError(t, err)
			roundtrip, err := data.BcsSerialize()
			require.NoError(t, err)
			require.Equal(t, wire, roundtrip)
			rebuilt, err := txBuilder.Transfer(args, &decoded)
			require.NoError(t, err)
			rebuiltBytes, err := rebuilt.Serialize()
			require.NoError(t, err)
			require.Equal(t, wire, rebuiltBytes)
			transaction := data.(*bcs.TransactionData__V1).Value
			require.Equal(t, tc.addressGas, len(transaction.GasData.Payment) == 0)
			if tc.addressGas {
				expiry := transaction.Expiration.(*bcs.TransactionExpiration__ValidDuring)
				require.Equal(t, input.CurrentEpoch, *expiry.MinEpoch)
				require.Equal(t, *expiry.MinEpoch, *expiry.MaxEpoch)
				require.Equal(t, input.AddressBalance.Nonce, expiry.Nonce)
			}
			ptb := transaction.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value
			var withdrawn uint64
			for _, arg := range ptb.Inputs {
				if withdrawal, ok := arg.(*bcs.CallArg__FundsWithdrawal); ok {
					require.False(t, withdrawal.Sponsor)
					withdrawn += withdrawal.Amount
				}
			}
			require.Equal(t, tc.withdrawal, withdrawn)
			sighashes, err := tx.Sighashes()
			require.NoError(t, err)
			count := 1
			if tc.sponsored {
				count = 2
			}
			require.Len(t, sighashes, count)
			require.False(t, input.SafeFromDoubleSend(&decoded))
			require.False(t, input.IndependentOf(&decoded))
		})
	}
}

func TestAddressBalanceFundingErrors(t *testing.T) {
	for _, tc := range []struct {
		name             string
		balance, amount  uint64
		failedSimulation bool
		want             string
	}{
		{"insufficient_transfer", 10_000_000, 11_000_000, false, "insufficient funds"},
		{"no_gas_remainder", 10_000_000, 10_000_000, false, "insufficient SUI for gas"},
		{"insufficient_estimated_gas", 10_000_000, 9_000_000, false, "insufficient SUI in gas funding source"},
		{"failed_simulation", 100_000_000, 10_000_000, true, "simulation failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := transferServer(t)
			server.balance = func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
				return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(tc.balance)}}, nil
			}
			server.coins = func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
				return &pb.ListOwnedObjectsResponse{}, nil
			}
			if tc.failedSimulation {
				server.simulate = func(context.Context, *pb.SimulateTransactionRequest) (*pb.SimulateTransactionResponse, error) {
					return &pb.SimulateTransactionResponse{Transaction: &pb.ExecutedTransaction{Effects: &pb.TransactionEffects{Status: &pb.ExecutionStatus{Success: proto.Bool(false)}}}}, nil
				}
			}
			c := newTestClient(t, server)
			args := buildertest.MustNewTransferArgs(c.Asset.Base(), sender, recipient, xc.NewAmountBlockchainFromUint64(tc.amount))
			_, err := c.FetchTransferInput(context.Background(), args)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestStakeWithAddressBalance(t *testing.T) {
	for _, sponsored := range []bool{false, true} {
		t.Run(fmt.Sprint(sponsored), func(t *testing.T) {
			server := transferServer(t)
			server.balance = func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
				return &pb.GetBalanceResponse{Balance: &pb.Balance{AddressBalance: proto.Uint64(10_000_000_000)}}, nil
			}
			server.coins = func(context.Context, *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
				return &pb.ListOwnedObjectsResponse{}, nil
			}
			c := newTestClient(t, server)
			options := []builder.BuilderOption{builder.OptionPublicKey(make([]byte, 32)), builder.OptionValidator(string(recipient)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(2_000_000_000))}
			if sponsored {
				options = append(options, builder.OptionFeePayer(sponsor, make([]byte, 32)))
			}
			args := buildertest.MustNewStakingArgs(xc.SUI, sender, options...)
			input, err := c.FetchStakingInput(context.Background(), args)
			require.NoError(t, err)
			b, err := sui.NewTxBuilder(c.Asset.Base())
			require.NoError(t, err)
			tx, err := b.Stake(args, input)
			require.NoError(t, err)
			hashes, err := tx.Sighashes()
			require.NoError(t, err)
			count := 1
			if sponsored {
				count = 2
			}
			require.Len(t, hashes, count)
			ptb := tx.(*sui.Tx).Tx.Value.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value
			require.Len(t, ptb.Commands, 2) // redeem_funds followed by request_add_stake; no unused coin
			require.Equal(t, "redeem_funds", string(ptb.Commands[0].(*bcs.Command__MoveCall).Value.Function))
			require.Equal(t, "request_add_stake", string(ptb.Commands[1].(*bcs.Command__MoveCall).Value.Function))
		})
	}
}
