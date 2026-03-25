package tx_input

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/proto"
)

// ComputePreparedTransactionHash returns the current local hash of a prepared
// transaction. This is the payload Canton transfer txs sign today.
func ComputePreparedTransactionHash(preparedTx *interactive.PreparedTransaction) ([]byte, error) {
	if preparedTx == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}

	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(preparedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal prepared transaction for hashing: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return digest[:], nil
}

// ValidatePreparedTransactionHash recomputes SHA-256 over the canonical proto
// encoding of PreparedTransaction and checks it matches the hash supplied by the
// prepare endpoint.
func ValidatePreparedTransactionHash(preparedTx *interactive.PreparedTransaction, expectedHash []byte) error {
	if len(expectedHash) == 0 {
		return fmt.Errorf("prepared transaction hash is empty")
	}

	digest, err := ComputePreparedTransactionHash(preparedTx)
	if err != nil {
		return err
	}
	if !bytes.Equal(digest, expectedHash) {
		return fmt.Errorf("prepared transaction hash mismatch: expected %x, got %x", expectedHash, digest)
	}

	return nil
}
