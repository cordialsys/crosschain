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
	// The estimated fee
	FeePerGram xc.AmountBlockchain `json:"fee_per_gram"`
	Mass       uint64              `json:"mass"`
	// we also include the network minimum fee, as the estimated fee may be lower than this
	MinFee xc.AmountBlockchain `json:"min_fee"`
}

type Utxo struct {
	TransactionId string              `json:"transaction_id"`
	Index         int                 `json:"index"`
	Amount        xc.AmountBlockchain `json:"amount"`
}

func (utxo *Utxo) Equals(other *Utxo) bool {
	return utxo.TransactionId == other.TransactionId && utxo.Index == other.Index
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
	// apply it to the mass or the min fee.
	// The min fee is often just '1', which isn't good for integer math.
	// So we apply the multiplier to the mass.
	mass := uint64(float64(input.Mass) * multiplier.InexactFloat64())
	input.Mass = mass

	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	mass := xc.NewAmountBlockchainFromUint64(input.Mass)
	feeEstimate := mass.Mul(&input.FeePerGram)

	if feeEstimate.Cmp(&input.MinFee) < 0 {
		return input.MinFee, ""
	}
	return feeEstimate, ""
}

func (input *TxInput) IndependentOf(otherI xc.TxInput) (independent bool) {
	other, ok := otherI.(*TxInput)
	if ok {
		for _, utxo := range input.Utxos {
			for _, otherUtxo := range other.Utxos {
				if utxo.Equals(&otherUtxo) {
					// not independent, spending the same utxo
					return false
				}
			}
		}
		return true
	}
	return false
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(other, input) {
		return false
	}
	// safe from double send only if spending the same utxo
	other, ok := other.(*TxInput)
	if !ok {
		return false
	}
	if input.IndependentOf(other) {
		return false
	}
	// all share the a dependency, can't double send
	return true
}
