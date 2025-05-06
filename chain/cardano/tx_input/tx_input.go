package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const FeeMargin = 500

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Utxos                   []types.Utxo `json:"utxos"`
	Slot                    uint64       `json:"slot"`
	Fee                     uint64       `json:"fee"`
	TransactionValidityTime uint64       `json:"transaction_validity_time"`
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

// Cardano does not support fee bidding
func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(input.Fee), ""
}

// check if any utxo is spent twice
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	cardanoOther, ok := other.(*TxInput)
	if !ok {
		return
	}

	for _, utxo1 := range input.Utxos {
		for _, utxo2 := range cardanoOther.Utxos {
			if utxo1.TxHash == utxo2.TxHash && utxo1.Index == utxo2.Index {
				// not independent
				return false
			}
		}
	}

	return true
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	// check if of the same types
	for _, other := range others {
		if _, ok := other.(*TxInput); !ok {
			return false
		}
	}

	// we are risking double send if we have any independent utxo's
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// conflicting utxo for all - we're safe
	return true
}
