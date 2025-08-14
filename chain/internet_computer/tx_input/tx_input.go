package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const SafetyTimeoutMargin = (24 * time.Hour) + (30 * time.Minute)

// TxInput for InternetComputerProtocol
type TxInput struct {
	xc.TxInputEnvelope
	Fee uint64 `json:"fee"`
	// UnixNano timestamp
	CreatedAtTime uint64             `json:"created_at_time"`
	Memo          uint64             `json:"memo"`
	Canister      xc.ContractAddress `json:"canister"`
	ICRC1Memo     *[]byte            `json:"icrc1_memo"`
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
	return xc.NewAmountBlockchainFromUint64(input.Fee), input.Canister
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if icpOther, ok := other.(*TxInput); ok {
		return input.CreatedAtTime != icpOther.CreatedAtTime
	}

	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	oldInput, ok := other.(*TxInput)
	if !ok {
		return true
	}

	// Transaqctions with `CreatedAtTime` > 24h cannot be submitted
	now := time.Now().UnixNano()
	diff := now - int64(oldInput.CreatedAtTime)
	if int64(diff) > SafetyTimeoutMargin.Nanoseconds() {
		return true
	}

	return false
}
