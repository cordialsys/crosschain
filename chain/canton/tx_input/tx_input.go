package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Canton holds the result of the prepare step in the
// interactive-submission (external-party signing) flow.
type TxInput struct {
	xc.TxInputEnvelope
	LedgerEnd            int64                             `json:"ledger_end"`
	PreparedTransaction  *interactive.PreparedTransaction  `json:"prepared_transaction,omitempty"`
	HashingSchemeVersion interactive.HashingSchemeVersion  `json:"hashing_scheme_version,omitempty"`
	// SubmissionId for deduplication (UUID)
	SubmissionId string `json:"submission_id"`
	// DeduplicationWindow controls how long Canton treats the submission ID as deduplicatable.
	DeduplicationWindow time.Duration `json:"deduplication_window"`
	// Decimals is the number of decimal places for the chain's native asset,
	// used to convert human-readable amounts in the prepared transaction to blockchain units.
	Decimals int32 `json:"decimals"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

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

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverCanton
}
func (input *TxInput) SetUnix(unix int64) {
	// TODO
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
	// Each Canton submission has a unique SubmissionId / command ID
	if cantonOther, ok := other.(*TxInput); ok {
		return cantonOther.SubmissionId != input.SubmissionId
	}
	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(other, input) {
		return false
	}
	if input.IndependentOf(other) {
		return false
	}
	// Same submission ID means deduplication will protect us
	return true
}
