package tx_input

import (
	"fmt"
	"math"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/cordialsys/crosschain/pkg/integer"
	"github.com/shopspring/decimal"
)

var _ xc.TxInput = &TxInput{}

// XlmTxInputGetter is a local interface for type-safe access to XLM tx input fields
// without requiring concrete type casts in IndependentOf/SafeFromDoubleSend.
type XlmTxInputGetter interface {
	GetXlmSequence() int64
	GetXlmFeePayerSequence() int64
}

func (input *TxInput) GetXlmSequence() int64 {
	if input.Sequence != 0 {
		return int64(input.Sequence)
	}
	return input.SequenceOld
}

func (input *TxInput) GetXlmFeePayerSequence() int64 {
	return int64(input.FeePayerSequence)
}

type TxInput struct {
	xc.TxInputEnvelope
	Passphrase string
	// SequenceOld is kept for backwards compatibility with the old JSON field name.
	SequenceOld int64 `json:"Sequence"`
	// Sequence uses integer.Int64 to survive JSON roundtrip without float64 precision loss.
	Sequence integer.Int64 `json:"sequence"`
	// Stellar requires the MaxFee specification, which defines the maximum amount
	// we are willing to spend on the transaction fee.
	MaxFee uint32
	// Specifies the duration for which a transaction remains valid after being submitted.
	TransactionActiveTime time.Duration
	MinLedgerSequence     int64
	// FeePayerSequence is the sequence number for the fee payer account,
	// used when a separate account pays the transaction fee.
	FeePayerSequence integer.Int64 `json:"fee_payer_sequence,omitempty"`
	// DestinationFunded indicates whether the destination account already exists on the network.
	// Stellar requires a CreateAccount operation for new accounts instead of Payment.
	DestinationFunded bool `json:"destination_funded,omitempty"`
	// NeedsCreateTrustline indicates that the sender needs a trustline for the token asset.
	// When true, a ChangeTrust operation is prepended to the transaction.
	NeedsCreateTrustline bool `json:"needs_create_trustline,omitempty"`
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput(passphrase string) *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverXlm,
		},
		Passphrase: passphrase,
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverXlm
}

// IndependentOf implements xc.TxInputConflicts.IndependentOf
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	otherInput, ok := other.(XlmTxInputGetter)
	if !ok {
		return false
	}
	// Compare both sender and fee-payer sequences when applicable
	if input.GetXlmFeePayerSequence() != 0 || otherInput.GetXlmFeePayerSequence() != 0 {
		return otherInput.GetXlmFeePayerSequence() != input.GetXlmFeePayerSequence()
	}
	return otherInput.GetXlmSequence() != input.GetXlmSequence()
}

// SafeFromDoubleSend implements xc.TxInputConflicts.SafeFromDoubleSend
func (input *TxInput) SafeFromDoubleSend(previousAttempt xc.TxInput) (safe bool) {
	if _, ok := previousAttempt.(XlmTxInputGetter); !ok {
		return false
	}

	if input.IndependentOf(previousAttempt) {
		return false
	}

	return true
}

// SetGasFeePriority implements xc.TxInputGasFeeMultiplier.SetGasFeePriority
func (input *TxInput) SetGasFeePriority(priority xc.GasFeePriority) error {
	multiplier, err := priority.GetDefault()
	if err != nil {
		return err
	}

	multipliedFee := multiplier.Mul(decimal.NewFromInt(int64(input.MaxFee)))
	asInt := multipliedFee.IntPart()
	if asInt > math.MaxUint32 {
		return fmt.Errorf("multiplied (x%s) max fee exceeds XLM limit, consider decreasing fee priority", multiplier.String())
	}

	input.MaxFee = uint32(asInt)
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(uint64(input.MaxFee)), ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	// currently getting a ~2 XLM fixed fee, not really accurate
	return false
}
