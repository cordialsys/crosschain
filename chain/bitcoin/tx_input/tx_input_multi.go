package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

// TxInput for Bitcoin
type MultiTransferInput struct {
	Inputs          []TxInput           `json:"inputs"`
	GasPricePerByte xc.AmountBlockchain `json:"gas_price_per_byte"`
	EstimatedSize   uint64              `json:"estimated_size"`
}

// This is a necessary interface so we can check conflicts between:
// * MultiTransferInput
// * TxInput
type UtxoGetter interface {
	GetUtxo() []Output
}

var _ xc.TxVariantInput = &MultiTransferInput{}
var _ xc.MultiTransferInput = &MultiTransferInput{}
var _ UtxoGetter = &MultiTransferInput{}

func NewMultiTransferInput() *MultiTransferInput {
	return &MultiTransferInput{}
}

func init() {
	registry.RegisterTxVariantInput(&MultiTransferInput{})
}

func (input *MultiTransferInput) GetDriver() xc.Driver {
	return xc.DriverBitcoin
}

func (input *MultiTransferInput) GetVariant() xc.TxVariantInputType {
	return xc.NewMultiTransferInputType(xc.DriverBitcoin, "native")
}

func (input *MultiTransferInput) MultiTransfer() {}

func (input *MultiTransferInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	gasPriceMultiplied := multiplier.Mul(decimal.NewFromBigInt(input.GasPricePerByte.Int(), 0)).BigInt()
	input.GasPricePerByte = xc.AmountBlockchain(*gasPriceMultiplied)
	return nil
}

func (txInput *MultiTransferInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gasPrice := txInput.GasPricePerByte
	txSize := txInput.EstimatedSize
	estimatedTxBytesLength := xc.NewAmountBlockchainFromUint64(
		txSize,
	)
	fee := gasPrice.Mul(&estimatedTxBytesLength)
	return fee, ""
}

func (input *MultiTransferInput) IndependentOf(other xc.TxInput) (independent bool) {
	if btcOther, ok := other.(UtxoGetter); ok {
		// check if any utxo are spent twice
		for _, utxo1 := range btcOther.GetUtxo() {
			for _, utxo2 := range input.GetUtxo() {
				if utxo1.Outpoint.Equals(&utxo2.Outpoint) {
					// not independent
					return false
				}
			}
		}
		return true
	}
	return
}

func (input *MultiTransferInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	// check that all other inputs are of the same type, so we can safely default-false
	for _, other := range others {
		if _, ok := other.(UtxoGetter); !ok {
			return false
		}
	}
	// any disjoint set of utxo's can risk double send
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// conflicting utxo for all - we're safe
	return true
}

func (txInput *MultiTransferInput) SumUtxo() *xc.AmountBlockchain {
	balance := xc.NewAmountBlockchainFromUint64(0)
	for _, input := range txInput.Inputs {
		balance = balance.Add(input.SumUtxo())
	}
	return &balance
}

func (input *MultiTransferInput) GetUtxo() []Output {
	size := 0
	for _, input := range input.Inputs {
		size += len(input.UnspentOutputs)
	}
	utxos := make([]Output, size)
	idx := 0
	for _, input := range input.Inputs {
		for _, utxo := range input.UnspentOutputs {
			utxos[idx] = utxo
			idx++
		}
	}
	return utxos
}
