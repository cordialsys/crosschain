package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

type NonceGetter interface {
	GetNonce() uint64
}

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Nonce uint64 `json:"nonce"`
	// Base58 encoded block hash
	BlockHash string `json:"block_hash"`
	// Deposit required for token transactions
	RequiredDepopsit xc.AmountBlockchain `json:"required_depopsit"`
	GasCost          xc.AmountBlockchain `json:"gas_cost"`
	FeeEstimation    xc.AmountBlockchain `json:"fee_estimation"`
}

var _ xc.TxInput = &TxInput{}
var _ NonceGetter = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverNear,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverNear
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	// unsupported
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return input.FeeEstimation, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) bool {
	old, ok := other.(NonceGetter)
	if ok {
		return old.GetNonce() != input.GetNonce()
	}
	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if other == nil {
		return true
	}
	o, ok := other.(NonceGetter)
	if ok {
		return input.Nonce == o.GetNonce()
	} else {
		return false
	}
}

func (input *TxInput) GetNonce() uint64 {
	return input.Nonce
}
