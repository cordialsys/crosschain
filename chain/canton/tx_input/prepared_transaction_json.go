package tx_input

import (
	"encoding/json"
	"fmt"

	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/encoding/protojson"
)

func MarshalPreparedTransactionJSON(prepared *interactive.PreparedTransaction) (json.RawMessage, error) {
	if prepared == nil {
		return nil, nil
	}
	bz, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(prepared)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal prepared transaction: %w", err)
	}
	return bz, nil
}

func UnmarshalPreparedTransactionJSON(data json.RawMessage) (*interactive.PreparedTransaction, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	var prepared interactive.PreparedTransaction
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &prepared); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prepared transaction: %w", err)
	}
	return &prepared, nil
}
