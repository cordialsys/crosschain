package tx_input

import (
	"math/big"

	"github.com/shopspring/decimal"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	// injectivecryptocodec "github.com/InjectiveLabs/sdk-go/chain/crypto/codec"
)

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
}

type CosmoAssetType string

// Cosmos assets can be managed by completely different modules (e.g. cosmwasm cw20, x/bank, etc)
var CW20 CosmoAssetType = "cw20"
var BANK CosmoAssetType = "bank"

// TxInput for Cosmos
type TxInput struct {
	xc.TxInputEnvelope
	AccountNumber         uint64  `json:"account_number,omitempty"`
	Sequence              uint64  `json:"sequence,omitempty"`
	FeePayerSequence      uint64  `json:"fee_payer_sequence,omitempty"`
	FeePayerAccountNumber uint64  `json:"fee_payer_account_number,omitempty"`
	GasLimit              uint64  `json:"gas_limit,omitempty"`
	GasPrice              float64 `json:"gas_price,omitempty"`
	TimeoutHeight         uint64  `json:"timeout_height"`

	AssetType CosmoAssetType `json:"asset_type,omitempty"`
	ChainId   string         `json:"chain_id,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ GetAccountInfo = &TxInput{}

type GetAccountInfo interface {
	GetAccountNumber() uint64
	GetSequence() uint64
}

func (input *TxInput) GetAccountNumber() uint64 {
	return input.AccountNumber
}

func (input *TxInput) GetSequence() uint64 {
	return input.Sequence
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverCosmos
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	input.GasPrice, _ = multiplier.Mul(decimal.NewFromFloat(input.GasPrice)).Float64()
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gasPrice := decimal.NewFromFloat(input.GasPrice)
	// Use big int to avoid casting uint64 to int64
	gasLimitInt := big.NewInt(0)
	gasLimitInt.SetUint64(input.GasLimit)
	gasLimit := decimal.NewFromBigInt(gasLimitInt, 0)

	// gasPrice * gasLimit
	totalSpend := gasPrice.Mul(gasLimit)
	totalSpendInt := xc.AmountBlockchain(*totalSpend.BigInt())

	// TODO: consider alt assets fees in cosmos
	return totalSpendInt, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if cosmosOther, ok := other.(GetAccountInfo); ok {
		// cosmos address could own multiple accounts too which each have independent sequence.
		return cosmosOther.GetAccountNumber() != input.GetAccountNumber() ||
			cosmosOther.GetSequence() != input.GetSequence()
	}
	return
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if _, ok := other.(GetAccountInfo); !ok {
		return false
	}
	// all same sequence means no double send
	if input.IndependentOf(other) {
		return false
	}
	// sequence all same - we're safe
	return true
}

// NewTxInput returns a new Cosmos TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverCosmos),
	}
}
