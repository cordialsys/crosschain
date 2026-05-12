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
var _ xc.TxInputWithUnix = &TxInput{}

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
	// MaxFee is the per-tx inclusion fee written into the tx envelope. We
	// populate it with the actual fee the network is expected to charge
	// (base_fee_in_stroops * num_operations from the latest ledger), so that
	// the on-chain charge and the inclusive-fee deduction line up.
	// ChainGasMultiplier can be used to add headroom for fee surges.
	MaxFee uint32
	// MinBalance is the minimum balance the source account must hold, derived
	// from (2 + subentry_count) * base_reserve. Used to drive the sweep logic.
	MinBalance xc.AmountBlockchain `json:"min_balance,omitempty"`
	// AccountMerge is set when the user is sweeping their full balance and the
	// account is eligible to be merged into the destination (subentry_count == 0
	// and destination is funded). The builder will then emit an AccountMerge
	// operation instead of Payment so the network reserve is released too.
	AccountMerge bool `json:"account_merge,omitempty"`
	// MustReserve is set when the user is sweeping but the account is not
	// eligible for AccountMerge. The minimum balance must remain in the source
	// account, so inclusive-fee spending deducts it in addition to the network
	// fee (see GetFeeLimit).
	MustReserve bool `json:"must_reserve,omitempty"`
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
	// SorobanResourceFee is the resource fee (in stroops) for Soroban InvokeHostFunction transactions.
	// It is added to MaxFee (the inclusion fee) to form the total transaction fee.
	SorobanResourceFee uint32 `json:"soroban_resource_fee,omitempty"`
	// Soroban resource limits for InvokeHostFunction transactions.
	SorobanInstructions  uint32 `json:"soroban_instructions,omitempty"`
	SorobanDiskReadBytes uint32 `json:"soroban_disk_read_bytes,omitempty"`
	SorobanWriteBytes    uint32 `json:"soroban_write_bytes,omitempty"`
	// SorobanTransactionData is the base64-encoded XDR SorobanTransactionData
	// returned by simulateTransaction. It carries the complete simulated
	// footprint and resource limits.
	SorobanTransactionData string `json:"soroban_transaction_data,omitempty"`
	// SorobanAuthorizationEntries are base64-encoded XDR
	// SorobanAuthorizationEntry values returned by simulateTransaction. Builders
	// use only matching RootInvocation.SubInvocations from these entries.
	SorobanAuthorizationEntries []string `json:"soroban_authorization_entries,omitempty"`

	Timestamp int64 `json:"timestamp,omitempty"`
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

	if input.SorobanResourceFee > 0 {
		multipliedResourceFee := multiplier.Mul(decimal.NewFromInt(int64(input.SorobanResourceFee)))
		asInt := multipliedResourceFee.IntPart()
		if asInt > math.MaxUint32 {
			return fmt.Errorf("multiplied (x%s) soroban resource fee exceeds limit", multiplier.String())
		}
		input.SorobanResourceFee = uint32(asInt)
	}

	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	totalFee := uint64(input.MaxFee) + uint64(input.SorobanResourceFee)
	if input.MustReserve {
		totalFee += input.MinBalance.Uint64()
	}
	return xc.NewAmountBlockchainFromUint64(totalFee), ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return true
}

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
}
