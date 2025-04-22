package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

// TxInput for Bitcoin
type MultiTransferInput struct {
	Inputs []*TxInput
	// UnspentOutputs  []Output            `json:"unspent_outputs"`
	// FromPublicKey   []byte              `json:"from_pubkey"`
	GasPricePerByte xc.AmountBlockchain `json:"gas_price_per_byte"`
	// Estimated size in bytes, per utxo that gets spent
	EstimatedSizePerSpentUtxo uint64 `json:"estimated_size_per_spent_utxo"`
}

var _ xc.TxVariantInput = &MultiTransferInput{}
var _ xc.MultiTransferInput = &MultiTransferInput{}

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

func (txInput *MultiTransferInput) GetEstimatedSizePerSpentUtxo() uint64 {
	if txInput.EstimatedSizePerSpentUtxo == 0 {
		log.WithField("driver", txInput.GetDriver()).Warn("estimated size per spent utxo not set")
		return 255
	}
	return txInput.EstimatedSizePerSpentUtxo
}

func (txInput *MultiTransferInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gasPrice := txInput.GasPricePerByte
	allUtxoCount := 0
	for _, input := range txInput.Inputs {
		allUtxoCount += len(input.UnspentOutputs)
	}
	estimatedTxBytesLength := xc.NewAmountBlockchainFromUint64(
		txInput.GetEstimatedSizePerSpentUtxo() * uint64(allUtxoCount),
	)
	fee := gasPrice.Mul(&estimatedTxBytesLength)
	return fee, ""
}

func (input *MultiTransferInput) IndependentOf(other xc.TxInput) (independent bool) {
	if btcOther, ok := other.(*MultiTransferInput); ok {
		for _, inputOther := range btcOther.Inputs {
			for _, input := range input.Inputs {
				// check if any utxo are spent twice
				for _, utxo1 := range inputOther.UnspentOutputs {
					for _, utxo2 := range input.UnspentOutputs {
						if utxo1.Outpoint.Equals(&utxo2.Outpoint) {
							// not independent
							return false
						}
					}
				}
			}
		}
		return true
	}
	return
}

func (input *MultiTransferInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
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
