package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
}

var _ xc.TxInput = &TxInput{}

func init() {
	// Uncomment this line to register the driver input for serialization/derserialization
	// registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: "INPUT_DRIVER_HERE",
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return "DRIVER HERE"
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	// multiply the gas price using the default, or apply a strategy according to the enum
	_ = multiplier
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// are these two transactions independent (e.g. different sequences & utxos & expirations?)
	// default false
	return
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	// safe from double send ?
	// default false
	return
}
