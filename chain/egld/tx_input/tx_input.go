package tx_input

import (
	"math/big"

	"github.com/shopspring/decimal"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for EGLD (MultiversX)
type TxInput struct {
	xc.TxInputEnvelope

	// Account nonce - incremented with each transaction
	Nonce uint64 `json:"nonce"`

	// Gas limit for the transaction
	GasLimit uint64 `json:"gas_limit"`

	// Gas price in wei (smallest denomination)
	GasPrice uint64 `json:"gas_price"`

	// Chain ID (e.g., "1" for mainnet, "D" for devnet, "T" for testnet)
	ChainID string `json:"chain_id"`

	// Transaction version (minimum is 1)
	Version uint32 `json:"version"`
}

var _ xc.TxInput = &TxInput{}
var _ GetAccountInfo = &TxInput{}

type GetAccountInfo interface {
	GetNonce() uint64
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func (input *TxInput) GetNonce() uint64 {
	return input.Nonce
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverElrond),
		Version:         1, // Default transaction version
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverElrond
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}

	// Multiply the gas price by the priority multiplier
	gasPriceDecimal := decimal.NewFromInt(int64(input.GasPrice))
	newGasPrice := multiplier.Mul(gasPriceDecimal)
	input.GasPrice = newGasPrice.BigInt().Uint64()

	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// Calculate max possible fee: gasLimit * gasPrice
	gasLimit := big.NewInt(0).SetUint64(input.GasLimit)
	gasPrice := big.NewInt(0).SetUint64(input.GasPrice)

	maxFee := big.NewInt(0).Mul(gasLimit, gasPrice)

	return xc.AmountBlockchain(*maxFee), ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// Transactions are independent if they have different nonces
	if egldOther, ok := other.(GetAccountInfo); ok {
		return egldOther.GetNonce() != input.GetNonce()
	}
	// Can't determine - default to not independent
	return false
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if _, ok := other.(GetAccountInfo); !ok {
		return false
	}

	// If transactions have different nonces, they won't conflict
	// Same nonce means one will replace the other (not safe)
	return input.IndependentOf(other)
}
