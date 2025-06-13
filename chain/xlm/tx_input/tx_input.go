package tx_input

import (
	"fmt"
	"math"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

var _ xc.TxInput = &TxInput{}

type TxInput struct {
	xc.TxInputEnvelope
	Passphrase string
	// Changes how sequence number is checked.
	// Is `Sequence == 0` then only transaction where
	// `SourceAccount.Sequence == tx.Sequence - 1` is allowed
	Sequence int64
	// Stellar requires the MaxFee specification, which defines the maximum amount
	// we are willing to spend on the transaction fee.
	MaxFee uint32
	// Specifies the duration for which a transaction remains valid after being submitted.
	TransactionActiveTime time.Duration
	MinLedgerSequence     int64
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
	if emvOther, ok := other.(*TxInput); ok {
		return emvOther.Sequence != input.Sequence
	}

	return false
}

// SafeFromDoubleSend implements xc.TxInputConflicts.SafeFromDoubleSend
func (input *TxInput) SafeFromDoubleSend(previousAttempt xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(previousAttempt, input) {
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
