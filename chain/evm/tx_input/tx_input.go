package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/utils"
	"github.com/shopspring/decimal"
)

// TxInput for EVM
type TxInput struct {
	xc.TxInputEnvelope
	utils.TxPriceInput
	Nonce    uint64 `json:"nonce,omitempty"`
	GasLimit uint64 `json:"gas_limit,omitempty"`
	// DynamicFeeTx
	GasTipCap xc.AmountBlockchain `json:"gas_tip_cap,omitempty"` // maxPriorityFeePerGas
	GasFeeCap xc.AmountBlockchain `json:"gas_fee_cap,omitempty"` // maxFeePerGas
	// GasPrice xc.AmountBlockchain `json:"gas_price,omitempty"` // wei per gas
	// Task params
	Params []string `json:"params,omitempty"`

	// For legacy implementation only
	GasPrice xc.AmountBlockchain `json:"gas_price,omitempty"` // wei per gas

	ChainId xc.AmountBlockchain `json:"chain_id,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPricing = &TxInput{}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverEVM,
		},
	}
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	multipliedTipCap := multiplier.Mul(decimal.NewFromBigInt(input.GasTipCap.Int(), 0)).BigInt()
	input.GasTipCap = xc.AmountBlockchain(*multipliedTipCap)

	if input.GasFeeCap.Cmp(&input.GasTipCap) < 0 {
		// increase max fee cap to accomodate tip if needed
		input.GasFeeCap = input.GasTipCap
	}

	// multiply the legacy gas price too
	multipliedLegacyGasPrice := multiplier.Mul(decimal.NewFromBigInt(input.GasPrice.Int(), 0)).BigInt()
	input.GasPrice = xc.AmountBlockchain(*multipliedLegacyGasPrice)
	return nil
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if evmOther, ok := other.(*TxInput); ok {
		return evmOther.Nonce != input.Nonce
	}
	return
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	// all same sequence means no double send
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// sequence all same - we're safe
	return true
}
