package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// Default gas price for Dusk. Units are LUX - minimum unit of Dusk.
// 1 LUX = 0.000_000_001 DUSK
const DEFAULT_GAS_PRICE = 1

// Dusk fee is capped by GasLimit * GasPrice.
// `(GasLimit - GasUsed) * GasPrice` is returned to specified account.
// `GasUsed` is the gas cost of the transaction.
type TxInput struct {
	xc.TxInputEnvelope
	Nonce uint64 `json:"nonce"`
	// GasLimit is the maximum amount of gas that can be used for the transaction
	GasLimit uint64 `json:"gas_limit"`
	// GasPrice is the amount of gas fee that user is willing to pay per gas
	GasPrice uint64 `json:"gas_price"`
	ChainId  uint8  `json:"chain_id"`
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverDusk,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverDusk
}

// Adjust `GasLimit` so it satisfies new `feePriority` without affecting `input.GasPrice`
func (input *TxInput) SetGasFeePriority(feePriority xc.GasFeePriority) error {
	multiplier, err := feePriority.GetDefault()
	if err != nil {
		return err
	}

	feeLimit := xc.NewAmountBlockchainFromUint64(input.GasLimit * input.GasPrice)
	var newFeeLimit xc.AmountBlockchain

	if feePriority.IsEnum() {
		floatMul, _ := multiplier.Float64()
		newFeeLimit = xc.MultiplyByFloat(feeLimit, floatMul)
	} else {
		hrFeeLimit, err := xc.NewAmountHumanReadableFromStr(multiplier.String())
		if err != nil {
			return fmt.Errorf("invalid multiplier: %w", err)
		}

		newFeeLimit = hrFeeLimit.ToBlockchain(9)
	}

	input.GasLimit = EstimateFeeLimit(newFeeLimit, xc.NewAmountBlockchainFromUint64(input.GasPrice)).Uint64()
	return nil
}

// get the max possible fee that could be spent on this transaction
func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(input.GasLimit * input.GasPrice), ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if evmOther, ok := other.(*TxInput); ok {
		return evmOther.Nonce != input.Nonce
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

func EstimateFeeLimit(feeLimit xc.AmountBlockchain, gasPrice xc.AmountBlockchain) xc.AmountBlockchain {
	gasLimit := feeLimit.Div(&gasPrice)
	return gasLimit
}
