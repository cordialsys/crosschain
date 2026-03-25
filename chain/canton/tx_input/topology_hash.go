package tx_input

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
)

const (
	topologyTransactionHashPurpose = 11
	topologyMultiHashPurpose       = 55
)

var sha256MultihashPrefix = []byte{0x12, 0x20}

func ComputeTopologyTransactionHash(serializedTx []byte) ([]byte, error) {
	if len(serializedTx) == 0 {
		return nil, fmt.Errorf("topology transaction is empty")
	}
	return computeCantonHash(topologyTransactionHashPurpose, serializedTx), nil
}

func ComputeTopologyMultiHash(serializedTxs [][]byte) ([]byte, error) {
	if len(serializedTxs) == 0 {
		return nil, fmt.Errorf("topology transactions are empty")
	}

	hashes := make([][]byte, 0, len(serializedTxs))
	for _, tx := range serializedTxs {
		hash, err := ComputeTopologyTransactionHash(tx)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}

	slices.SortFunc(hashes, func(a, b []byte) int {
		return slices.Compare(a, b)
	})

	combined := make([]byte, 4)
	binary.BigEndian.PutUint32(combined, uint32(len(hashes)))
	for _, hash := range hashes {
		size := make([]byte, 4)
		binary.BigEndian.PutUint32(size, uint32(len(hash)))
		combined = append(combined, size...)
		combined = append(combined, hash...)
	}

	return computeCantonHash(topologyMultiHashPurpose, combined), nil
}

func computeCantonHash(purpose uint32, content []byte) []byte {
	payload := make([]byte, 4+len(content))
	binary.BigEndian.PutUint32(payload[:4], purpose)
	copy(payload[4:], content)
	sum := sha256.Sum256(payload)

	result := make([]byte, 0, len(sha256MultihashPrefix)+len(sum))
	result = append(result, sha256MultihashPrefix...)
	result = append(result, sum[:]...)
	return result
}
