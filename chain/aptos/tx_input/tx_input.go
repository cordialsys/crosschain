package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

type TxInput struct {
	xc.TxInputEnvelope
	SequenceNumber uint64 `json:"sequence_number,omitempty"`
	GasLimit       uint64 `json:"gas_limit,omitempty"`
	GasPrice       uint64 `json:"gas_price,omitempty"`
	Timestamp      uint64 `json:"timestamp,omitempty"`
	ChainId        int    `json:"chain_id,omitempty"`
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverAptos,
		},
	}
}
func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverAptos
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	input.GasPrice = multiplier.Mul(decimal.NewFromInt(int64(input.GasPrice))).BigInt().Uint64()
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gasLimit := xc.NewAmountBlockchainFromUint64(input.GasLimit)
	gasPrice := xc.NewAmountBlockchainFromUint64(input.GasPrice)
	maxFeeSpend := gasLimit.Mul(&gasPrice)
	return maxFeeSpend, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if aptosOther, ok := other.(*TxInput); ok {
		return aptosOther.SequenceNumber != input.SequenceNumber
	}
	return
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(other, input) {
		return false
	}
	// all same sequence means no double send
	if input.IndependentOf(other) {
		return false
	}
	// sequence all same - we're safe
	return true
}
