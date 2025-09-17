package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const UsdcTokenId = "USDC:0x6d1e7cde53ba9467b783cb7c530ce054"
const SafetyTimeoutMargin = (48 * time.Hour)

// TxInput for Hyperliquid
type TxInput struct {
	xc.TxInputEnvelope
	TransactionTime int64 `json:"transaction_time"`
	// Token decimals
	Decimals int32 `json:"decimals"`
	// Token
	Token xc.ContractAddress `json:"token"`
	// "Mainnet" or "Testnet"
	HyperliquidChain string
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverHyperliquid,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverHyperliquid
}

// No gas priority in hyperliquid
func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	return nil
}

// Most SpotTransfers are free on hyperliquid, with hardcoded 1 USDC fee for Arbitrum withdrawals
// and first spot transfer
func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(100_000_000), UsdcTokenId
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if hypeOther, ok := other.(*TxInput); ok {
		return input.TransactionTime != hypeOther.TransactionTime
	}

	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	oldInput, ok := other.(*TxInput)
	if !ok {
		return true
	}

	diff := input.TransactionTime - oldInput.TransactionTime
	if diff > int64(SafetyTimeoutMargin.Seconds()) {
		return true
	}

	return false
}
