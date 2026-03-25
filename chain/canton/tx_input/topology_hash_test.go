package tx_input

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeTopologyTransactionHash(t *testing.T) {
	t.Parallel()

	hash, err := ComputeTopologyTransactionHash([]byte{0x01, 0x02, 0x03})
	require.NoError(t, err)
	require.Equal(t, "1220c1467c073293cec489633f24df12269d37f5fc14c5e7793119703965b001b751", hex.EncodeToString(hash))
}

func TestComputeTopologyMultiHash(t *testing.T) {
	t.Parallel()

	hash1, err := ComputeTopologyMultiHash([][]byte{{0x01, 0x02}, {0x03, 0x04}})
	require.NoError(t, err)

	hash2, err := ComputeTopologyMultiHash([][]byte{{0x03, 0x04}, {0x01, 0x02}})
	require.NoError(t, err)

	require.Equal(t, hash1, hash2)
	require.Equal(t, "122086be10f2f6a61410ec823c4350eb8eef35b66481a0dd080f2607f15f7d3b57ef", hex.EncodeToString(hash1))
}

func TestComputeTopologyMultiHashFailures(t *testing.T) {
	t.Parallel()

	_, err := ComputeTopologyMultiHash(nil)
	require.ErrorContains(t, err, "topology transactions are empty")

	_, err = ComputeTopologyMultiHash([][]byte{{}})
	require.ErrorContains(t, err, "topology transaction is empty")
}
