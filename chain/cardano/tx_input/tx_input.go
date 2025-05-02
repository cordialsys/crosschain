package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Utxos []types.Utxo `json:"utxos"`
	Slot  uint64       `json:"slot"`
	// FixedFee is defined as the minimum fee for a transaction
	FixedFee xc.AmountBlockchain `json:"min_fee_b"`
	// FeePerByte is the fee per transaction byte
	FeePerByte xc.AmountBlockchain `json:"min_fee_a"`
	// MinUtxo is the amount of minimum utxo value for ADA only transactions
	MinUtxo xc.AmountBlockchain `json:"min_utxo"`
	// CoinsPerUtxoWord is the price per utxo word for multi-asset transactions
	CoinsPerUtxoWord xc.AmountBlockchain `json:"price_per_utxo_word"`
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverCardano,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverCardano
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
