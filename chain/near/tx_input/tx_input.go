package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// Function calls need an execution budget in addition to protocol action fees.
const DefaultFunctionCallGas uint64 = 30_000_000_000_000

type NonceGetter interface {
	GetNonce() uint64
}

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Nonce uint64 `json:"nonce"`
	// Base58 encoded block hash
	BlockHash string `json:"block_hash"`
	// Deprecated: older clients populated this after checking the sender's token balance.
	// Retained for input compatibility; it must not trigger receiver registration.
	RequiredDepopsit xc.AmountBlockchain `json:"required_depopsit"`
	// A non-nil deposit requests receiver registration, even when the minimum is zero.
	StorageDeposit *xc.AmountBlockchain `json:"storage_deposit,omitempty"`
	StorageGasCost xc.AmountBlockchain  `json:"storage_gas_cost,omitempty"`
	// The separate 1 yoctoNEAR deposit attached to ft_transfer (zero for native transfers).
	TransferDeposit xc.AmountBlockchain `json:"transfer_deposit,omitempty"`
	GasCost         xc.AmountBlockchain `json:"gas_cost"`
	FeeEstimation   xc.AmountBlockchain `json:"fee_estimation"`
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
	fee := input.FeeEstimation.Add(&input.TransferDeposit)
	if input.StorageDeposit != nil {
		fee = fee.Add(input.StorageDeposit)
	}
	return fee, ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return true
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
