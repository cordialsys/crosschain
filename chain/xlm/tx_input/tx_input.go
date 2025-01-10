package tx_input

import (
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithMemo = &TxInput{}

type TxInput struct {
	xc.TxInputEnvelope
	Passphrase string
	// Changes how sequence number is checked.
	// Is `Sequence == 0` then only transaction where
	// `SourceAccount.Sequence == tx.Sequence - 1` is allowed
	Sequence int64
	// Min fee for stellar transactions
	MinFee uint32
	// Base fee for stellar transactions
	BaseFee uint32
	// Max amount of fee we are willing to pay in total
	MaxFee uint32
	// Time for which the transaction will be considered valid
	TransactionActiveTime time.Duration
	MinLedgerSequence     int64
	// Transaction memo
	Memo string
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
func (input *TxInput) SafeFromDoubleSend(previousAttempts ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, previousAttempts...) {
		return false
	}

	for _, other := range previousAttempts {
		if input.IndependentOf(other) {
			return false
		}
	}

	return true
}

// SetGasFeePriority implements xc.TxInputGasFeeMultiplier.SetGasFeePriority
func (input *TxInput) SetGasFeePriority(priority xc.GasFeePriority) error {
	multiplier, err := priority.GetDefault()
	if err != nil {
		return err
	}

	// Multiply the BaseFee and check if it's valid
	newFee := input.BaseFee * uint32(multiplier.BigInt().Uint64())
	if newFee < input.MinFee {
		return fmt.Errorf(
			"calculated gas(%d) is lower than minimal allowed gas(%d)",
			newFee,
			input.MinFee,
		)
	}

	input.MaxFee = input.MinFee * uint32(multiplier.BigInt().Uint64())
	return nil
}

// SetMemo implements xc.TxInputWithMemo.SetMemo
func (input *TxInput) SetMemo(memo string) {
	input.Memo = memo
}
