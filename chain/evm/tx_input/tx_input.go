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
	Nonce       uint64     `json:"nonce,omitempty"`
	FromAddress xc.Address `json:"from_address,omitempty"`
	GasLimit    uint64     `json:"gas_limit,omitempty"`
	// DynamicFeeTx
	GasTipCap xc.AmountBlockchain `json:"gas_tip_cap,omitempty"` // maxPriorityFeePerGas
	GasFeeCap xc.AmountBlockchain `json:"gas_fee_cap,omitempty"` // maxFeePerGas
	L1Fee     xc.AmountBlockchain `json:"l1_fee,omitempty"`
	// GasPrice xc.AmountBlockchain `json:"gas_price,omitempty"` // wei per gas
	// Task params
	Params []string `json:"params,omitempty"`

	// For legacy implementation only
	GasPrice xc.AmountBlockchain `json:"gas_price,omitempty"` // wei per gas

	ChainId xc.AmountBlockchain `json:"chain_id,omitempty"`

	// For eip7702 transactions
	BasicSmartAccountNonce uint64     `json:"basic_smart_account_nonce,omitempty"`
	FeePayerAddress        xc.Address `json:"fee_payer_address,omitempty"`
	FeePayerNonce          uint64     `json:"fee_payer_nonce,omitempty"`

	// legacy only
	Prices []*Price `json:"prices,omitempty"`
}

func (input *TxInput) GetNonce() uint64 {
	return input.Nonce
}

func (input *TxInput) GetFromAddress() string {
	return string(input.FromAddress)
}

func (input *TxInput) GetFeePayerNonce() uint64 {
	return input.FeePayerNonce
}

func (input *TxInput) GetFeePayerAddress() string {
	return string(input.FeePayerAddress)
}

type ToTxInput interface {
	// For ensuring compatibility with chains upgraded from evm-legacy driver
	ToTxInput() *TxInput
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
	multipliedMaxCap := multiplier.Mul(decimal.NewFromBigInt(input.GasFeeCap.Int(), 0)).BigInt()
	input.GasTipCap = xc.AmountBlockchain(*multipliedTipCap)
	input.GasFeeCap = xc.AmountBlockchain(*multipliedMaxCap)

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
	maxFeeSpend := dynamicMaxFeeSpend
	if legacyMaxFeeSpend.Cmp(&maxFeeSpend) > 0 {
		maxFeeSpend = legacyMaxFeeSpend
	}
	maxFeeSpend = maxFeeSpend.Add(&input.L1Fee)
	return maxFeeSpend, ""
}

type GetAccountInfo interface {
	GetNonce() uint64
	GetFromAddress() string

	GetFeePayerNonce() uint64
	GetFeePayerAddress() string
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if toTxInput, ok := other.(ToTxInput); ok {
		other = toTxInput.ToTxInput()
	}
	evmOther, ok := other.(GetAccountInfo)
	if !ok {
		return false
	}

	if input.GetFeePayerAddress() != "" || input.GetFeePayerNonce() != 0 {
		if evmOther.GetNonce() == input.Nonce && strings.EqualFold(evmOther.GetFromAddress(), input.GetFromAddress()) {
			return false
		}
		// Should not sign multiple tx for the same fee-payer nonce.
		if strings.EqualFold(evmOther.GetFeePayerAddress(), input.GetFeePayerAddress()) &&
			evmOther.GetFeePayerNonce() == input.GetFeePayerNonce() {
			return false
		}
	} else {
		if evmOther.GetNonce() == input.Nonce {
			return false
		}
	}
	return true
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if toTxInput, ok := other.(ToTxInput); ok {
		other = toTxInput.ToTxInput()
	}
	if !xc.IsTypeOf(other, input, MultiTransferInput{}, BatchDepositInput{}, ExitRequestInput{}) {
		return false
	}
	// all same sequence means no double send
	if input.IndependentOf(other) {
		return false
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
