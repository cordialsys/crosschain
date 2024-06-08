package newchain

import (
	xc "github.com/cordialsys/crosschain"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
}

var _ xc.TxInput = &TxInput{}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: "INPUT_DRIVER_HERE",
		},
	}
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
