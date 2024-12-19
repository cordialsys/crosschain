package tx_input

import (
	// "encoding/base64"
	// "encoding/hex"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithMemo = &TxInput{}

type TxInput struct {
	xc.TxInputEnvelope
	Sequence int64
	// Minimum fee that will be paid per transaction operation
	BaseFee uint32
	// Max amount of fee we are willing to pay in total
	MaxFee            uint32
	MinLedgerSequence int64
	Memo              string
	PublicKey         []byte
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverXlm,
		},
	}
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverXlm
}

// IndependentOf implements xc.TxInputConflicts.IndependentOf
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	return true
}

// SafeFromDoubleSend implements xc.TxInputConflicts.SafeFromDoubleSend
func (input *TxInput) SafeFromDoubleSend(previousAttempts ...xc.TxInput) (safe bool) {
	return true
}

// SetGasFeePriority implements xc.TxInputGasFeeMultiplier.SetGasFeePriority
func (input *TxInput) SetGasFeePriority(priority xc.GasFeePriority) error {
	return nil
}

// SetMemo implements xc.TxInputWithMemo.SetMemo
func (input *TxInput) SetMemo(string) {
}
