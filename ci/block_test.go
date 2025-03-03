//go:build ci

package ci

import (
	"context"
	"flag"
	"testing"
	"time"

	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"

	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/stretchr/testify/require"
)

func TestFetchBlock(t *testing.T) {
	ctx := context.Background()
	flag.Parse()

	rpcArgs := &setup.RpcArgs{
		Chain:     chain,
		Rpc:       rpc,
		Network:   network,
		Overrides: map[string]*setup.ChainOverride{},
		Algorithm: algorithm,
	}

	xcFactory, err := setup.LoadFactory(rpcArgs)
	require.NoError(t, err, "Failed loading factory")
	chainConfig, err := setup.LoadChain(xcFactory, rpcArgs.Chain)
	require.NoError(t, err, "Failed loading chain config")
	client, err := xcFactory.NewClient(chainConfig)
	require.NoError(t, err, "Failed creating client")

	// get latest (attempt multiple times, as head of chain can be flakey)
	var latest *xcclient.BlockWithTransactions
	for range 8 {
		latest, err = client.FetchBlock(ctx, xcclient.LatestHeight())
		if err != nil {
			logrus.WithError(err).Warn("could not fetch latest block, retrying...")
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	require.NoError(t, err, "could not fetch latest block")

	// get by specific height
	block, err := client.FetchBlock(ctx, xcclient.AtHeight(latest.Height.Uint64()))
	require.NoError(t, err, "could not fetch specific block")

	require.NotEqualValues(t, 0, block.Height)
	require.Equal(t, latest.Height, block.Height)

	require.NotEmpty(t, latest.Hash, "empty block hash from latest block")
	require.NotEmpty(t, block.Hash, "empty block hash from current block")
}
