package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"github.com/stretchr/testify/require"
)

func stakingRegressionClient(t *testing.T, principal, rewards uint64) *Client {
	t.Helper()
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID any `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": []any{map[string]any{"validatorAddress": recipient, "stakingPool": sponsor, "stakes": []any{map[string]any{"stakedSuiId": fmt.Sprintf("0x%064x", 9), "stakeRequestEpoch": "40", "stakeActiveEpoch": "41", "principal": fmt.Sprint(principal), "status": "Active", "estimatedReward": fmt.Sprint(rewards)}}}}}))
	}))
	t.Cleanup(httpServer.Close)
	server := transferServer(t)
	server.object = func(_ context.Context, r *pb.GetObjectRequest) (*pb.GetObjectResponse, error) {
		return &pb.GetObjectResponse{Object: coinObject(9, principal)}, nil
	}
	return newTestClient(t, server, func(cfg *xc.ChainConfig) { cfg.SecondaryURL = httpServer.URL })
}

func TestValidUnstakingSplitsAndFilters(t *testing.T) {
	for _, tc := range []struct {
		name           string
		amount, reward uint64
		validator      bool
	}{
		{"partial_below_half", 3_000_000_000, 0, false},
		{"partial_above_half", 7_000_000_000, 0, false},
		{"partial_with_rewards", 3_000_000_000, 1_000_000_000, false},
		{"full_without_filter", 10_000_000_000, 0, false},
		{"full_with_validator", 10_000_000_000, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := stakingRegressionClient(t, 10_000_000_000, tc.reward)
			opts := []builder.BuilderOption{builder.OptionPublicKey(make([]byte, 32)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(tc.amount))}
			if tc.validator {
				opts = append(opts, builder.OptionValidator(string(recipient)))
			}
			args := buildertest.MustNewStakingArgs(xc.SUI, sender, opts...)
			_, err := c.FetchUnstakingInput(context.Background(), args)
			require.NoError(t, err)
		})
	}
}

func TestSponsoredCoinOnlyStakeSignatures(t *testing.T) {
	server := transferServer(t)
	original := server.coins
	server.coins = func(ctx context.Context, r *pb.ListOwnedObjectsRequest) (*pb.ListOwnedObjectsResponse, error) {
		out, err := original(ctx, r)
		if len(out.Objects) > 1 {
			out.Objects = out.Objects[:1]
		}
		return out, err
	}
	c := newTestClient(t, server)
	args := buildertest.MustNewStakingArgs(xc.SUI, sender, builder.OptionPublicKey(make([]byte, 32)), builder.OptionValidator(string(recipient)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(2_000_000_000)), builder.OptionFeePayer(sponsor, make([]byte, 32)))
	in, err := c.FetchStakingInput(context.Background(), args)
	require.NoError(t, err)
	require.Nil(t, in.(*sui.StakingInput).AddressBalance)
	tb, err := sui.NewTxBuilder(c.Asset.Base())
	require.NoError(t, err)
	tx, err := tb.Stake(args, in)
	require.NoError(t, err)
	sigs, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, sigs, 2)
}

func TestUnstakeNeedsOnlyGas(t *testing.T) {
	c := stakingRegressionClient(t, 10_000_000_000, 0)
	// The RPC server used by stakingRegressionClient has several coins. Build an input
	// directly to exercise the equivalent single gas coin / zero address balance case.
	base, err := c.fetchBaseInput(context.Background(), sui.NativeCoin, sender, "")
	require.NoError(t, err)
	base.AddressBalance = nil
	base.Coins = nil
	require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"coinObjectId":"0x%064x","version":"123","digest":%q,"balance":"100000000","coinType":%q}`, 1, testDigest, sui.NativeCoin)), &base.GasCoin))
	base.GasBudget = 4_000_000
	input := &sui.UnstakingInput{TxInput: *base, StakesToUnstake: []sui.Stake{{Principal: xc.NewAmountBlockchainFromUint64(10_000_000_000), ObjectId: fmt.Sprintf("0x%064x", 9), Version: 123, Digest: testDigest}}}
	args := buildertest.MustNewStakingArgs(xc.SUI, sender, builder.OptionPublicKey(make([]byte, 32)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(10_000_000_000)))
	tb, err := sui.NewTxBuilder(c.Asset.Base())
	require.NoError(t, err)
	_, err = tb.Unstake(args, input)
	require.NoError(t, err)
}
