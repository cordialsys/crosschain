package tx_input

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

// TxInput for EVM
type TxInput struct {
	xc.TxInputEnvelope
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

	// legacy only
	Prices []*Price `json:"prices,omitempty"`
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&BatchDepositInput{})
	registry.RegisterTxVariantInput(&ExitRequestInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverEVM,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverEVM
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

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gasLimit := xc.NewAmountBlockchainFromUint64(input.GasLimit)

	legacyMaxFeeSpend := input.GasPrice.Mul(&gasLimit)
	dynamicMaxFeeSpend := input.GasFeeCap.Mul(&gasLimit)

	// use larger of the two
	if legacyMaxFeeSpend.Cmp(&dynamicMaxFeeSpend) > 0 {
		return legacyMaxFeeSpend, ""
	} else {
		return dynamicMaxFeeSpend, ""
	}
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

// Pricing methods are no longer used, except for a prior wormhole-bridging experiment.
type Price struct {
	Contract string                 `json:"contract"`
	Chain    xc.NativeAsset         `json:"chain"`
	PriceUsd xc.AmountHumanReadable `json:"price_usd"`
}

func (input *TxInput) SetUsdPrice(chain xc.NativeAsset, contract string, priceUsd xc.AmountHumanReadable) {
	// normalize the contract
	contract = strings.ToLower(contract)
	// remove any existing
	input.removeUsdPrice(chain, contract)
	// add new
	input.Prices = append(input.Prices, &Price{
		Contract: contract,
		Chain:    chain,
		PriceUsd: priceUsd,
	})
}

func (input *TxInput) GetUsdPrice(chain xc.NativeAsset, contract string) (xc.AmountHumanReadable, bool) {
	contract = strings.ToLower(contract)
	for _, price := range input.Prices {
		if price.Chain == chain && price.Contract == contract {
			return price.PriceUsd, true
		}
	}
	return xc.AmountHumanReadable{}, false
}

func (input *TxInput) removeUsdPrice(chain xc.NativeAsset, contract string) {
	for i, price := range input.Prices {
		if price.Chain == chain && price.Contract == contract {
			input.Prices = append(input.Prices[:i], input.Prices[i+1:]...)
			return
		}
	}
}
