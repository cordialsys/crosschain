package tx_input

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeTopologyMultiHashFailures(t *testing.T) {
	t.Parallel()

	_, err := ComputeTopologyMultiHash(nil)
	require.ErrorContains(t, err, "topology transactions are empty")

	_, err = ComputeTopologyMultiHash([][]byte{{}})
	require.ErrorContains(t, err, "topology transaction is empty")
}

func TestComputeTopologyMultiHash_LiveVector(t *testing.T) {
	t.Parallel()

	vector := mustLoadLiveTopologyVector(t)
	hash, err := ComputeTopologyMultiHash(vector.topologyTransactions)
	require.NoError(t, err)
	require.Equal(t, vector.multiHash, hash)
}

type liveTopologyVector struct {
	multiHash            []byte
	topologyTransactions [][]byte
}

func mustLoadLiveTopologyVector(t *testing.T) liveTopologyVector {
	t.Helper()

	data, err := os.ReadFile("testdata/live_topology_vector.json")
	require.NoError(t, err)

	var fixture struct {
		MultiHash            string   `json:"multi_hash"`
		TopologyTransactions []string `json:"topology_transactions"`
	}
	require.NoError(t, json.Unmarshal(data, &fixture))

	multiHash, err := hex.DecodeString(fixture.MultiHash)
	require.NoError(t, err)

	topologyTransactions := make([][]byte, 0, len(fixture.TopologyTransactions))
	for _, encoded := range fixture.TopologyTransactions {
		tx, err := hex.DecodeString(encoded)
		require.NoError(t, err)
		topologyTransactions = append(topologyTransactions, tx)
	}

	return liveTopologyVector{
		multiHash:            multiHash,
		topologyTransactions: topologyTransactions,
	}
}
