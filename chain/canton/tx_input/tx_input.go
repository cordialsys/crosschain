package tx_input

import (
	"encoding/json"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Canton holds the result of the prepare step in the
// interactive-submission (external-party signing) flow.
type TxInput struct {
	xc.TxInputEnvelope
	LedgerEnd            int64                            `json:"ledger_end"`
	PreparedTransaction  *interactive.PreparedTransaction `json:"prepared_transaction,omitempty"`
	HashingSchemeVersion interactive.HashingSchemeVersion `json:"hashing_scheme_version,omitempty"`
	// SubmissionId for deduplication (UUID)
	SubmissionId string `json:"submission_id"`
	// DeduplicationWindow controls how long Canton treats the submission ID as deduplicatable.
	DeduplicationWindow time.Duration `json:"deduplication_window"`
	// Decimals is the number of decimal places for the chain's native asset,
	// used to convert human-readable amounts in the prepared transaction to blockchain units.
	Decimals int32 `json:"decimals"`
	// ContractAddress is set when a native Canton transfer is prepared through
	// the token-standard TransferFactory path, so transaction validation can
	// enforce the expected instrument.
	ContractAddress xc.ContractAddress `json:"contract_address,omitempty"`
}

type txInputInner TxInput

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}
var _ cantonConflictInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverCanton,
		},
	}
}

// We have to use custom marshalling/unmarshalling for the prepared transaction,
// because the normal json.Marshal will not properly handle the protobuf oneof fields.
// we need to manually use protojson instead for it.
func (input *TxInput) MarshalJSON() ([]byte, error) {
	var prepared json.RawMessage
	if input.PreparedTransaction != nil {
		bz, err := MarshalPreparedTransactionJSON(input.PreparedTransaction)
		if err != nil {
			return nil, err
		}
		prepared = bz
	}
	type txInputJSON struct {
		*txInputInner
		PreparedTransaction json.RawMessage `json:"prepared_transaction,omitempty"`
	}
	return json.Marshal(txInputJSON{
		txInputInner:        (*txInputInner)(input),
		PreparedTransaction: prepared,
	})
}

func (input *TxInput) UnmarshalJSON(data []byte) error {
	type txInputJSON struct {
		*txInputInner
		PreparedTransaction json.RawMessage `json:"prepared_transaction,omitempty"`
	}
	wire := txInputJSON{
		txInputInner: (*txInputInner)(input),
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	input.PreparedTransaction = nil
	if len(wire.PreparedTransaction) > 0 && string(wire.PreparedTransaction) != "null" {
		prepared, err := UnmarshalPreparedTransactionJSON(wire.PreparedTransaction)
		if err != nil {
			return err
		}
		input.PreparedTransaction = prepared
	}
	return nil
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverCanton
}
func (input *TxInput) SetUnix(unix int64) {
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	// Canton does not use gas fees in the traditional sense
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(0), ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return true
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	return cantonIndependentOf(input, other)
}

func (input *TxInput) ConflictContractIDs() []string {
	return input.cantonConflictContractIDs()
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	return cantonSafeFromDoubleSend(input, other)
}

func (input *TxInput) cantonSubmissionID() string {
	if input == nil {
		return ""
	}
	return input.SubmissionId
}

func (input *TxInput) cantonConflictContractIDsKnown() bool {
	return input != nil && input.PreparedTransaction != nil
}

func (input *TxInput) cantonConflictContractIDs() []string {
	if input == nil {
		return nil
	}
	return consumingContractIDs(input.PreparedTransaction)
}

func (input *TxInput) cantonConflictKey() string {
	return ""
}
