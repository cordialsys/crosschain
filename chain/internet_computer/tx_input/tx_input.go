package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Fee           uint64
	CreatedAtTime uint64
	Memo          uint64
	Canister      xc.ContractAddress
	ICRC1Memo     *[]byte
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverInternetComputerProtocol,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverInternetComputerProtocol
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	// multiply the gas price using the default, or apply a strategy according to the enum
	_ = multiplier
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(input.Fee), ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if icpOther, ok := other.(*TxInput); ok {
		return input.CreatedAtTime != icpOther.CreatedAtTime
	}

	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(other, input) {
		return false
	}

	if input.IndependentOf(other) {
		return false
	}

	return true
}
