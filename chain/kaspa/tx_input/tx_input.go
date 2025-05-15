package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Address xc.Address `json:"address"`
	Utxos   []Utxo     `json:"utxo"`
}

type Utxo struct {
	TransactionId string              `json:"transaction_id"`
	Index         int                 `json:"index"`
	Amount        xc.AmountBlockchain `json:"amount"`
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverKaspa,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverKaspa
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

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// get the max possible fee that could be spent on this transaction
	return xc.NewAmountBlockchainFromUint64(0), ""
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
