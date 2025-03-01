package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// Filecoin TxInput
type TxInput struct {
	xc.TxInputEnvelope
	// Nonce of the account, incremented for each transaction
	Nonce uint64
	// GasLimit is the maximum amount of gas that can be used for the transaction
	GasLimit uint64
	// GasFeeCap is the maximum amount of gas fee that user is willing to pay
	GasFeeCap xc.AmountBlockchain
	// GasPremium is the amount of gas fee that user is willing to pay
	// per unit of gas
	GasPremium xc.AmountBlockchain
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverFilecoin,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverFilecoin
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}

	xcMultiplier := xc.AmountBlockchain(*multiplier.BigInt())
	input.GasFeeCap = input.GasFeeCap.Mul(&xcMultiplier)
	input.GasPremium = input.GasPremium.Mul(&xcMultiplier)
	return nil
}

func (input *TxInput) GetMaxFee() (xc.AmountBlockchain, xc.ContractAddress) {
	gasLimit := xc.NewAmountBlockchainFromUint64(input.GasLimit)
	maxFeeSpend := input.GasFeeCap.Mul(&gasLimit)
	return maxFeeSpend, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if emvOther, ok := other.(*TxInput); ok {
		return emvOther.Nonce != input.Nonce
	}
	return false
}

func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}

	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	return true
}
