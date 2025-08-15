package tx_input

import (
	"encoding/hex"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/internet_computer/agent"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const TransactionExpiration = 5 * time.Minute
const SafetyTimeoutMargin = TransactionExpiration + 5*time.Minute

// TxInput for InternetComputerProtocol
type TxInput struct {
	xc.TxInputEnvelope
	Fee uint64 `json:"fee"`
	// Unix second timestamp
	CreateTime int64              `json:"create_time"`
	Memo       uint64             `json:"memo"`
	Canister   xc.ContractAddress `json:"canister"`
	ICRC1Memo  *[]byte            `json:"icrc1_memo"`
	// encoded as hex
	Nonce string `json:"nonce"`
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
		return input.CreateTime != icpOther.CreateTime
	}

	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	oldInput, ok := other.(*TxInput)
	if !ok {
		return true
	}

	// Transaqctions with `CreatedAtTime` > 24h cannot be submitted
	diff := input.CreateTime - oldInput.CreateTime
	if diff > int64(SafetyTimeoutMargin.Seconds()) {
		return true
	}

	return false
}

func (input *TxInput) GetNonce() agent.Nonce {
	nonceInput, err := hex.DecodeString(input.Nonce)
	if err != nil {
		return agent.Nonce{}
	}
	// since nonce can be variable length, we just consume what need
	var nonce agent.Nonce
	for i := 0; i < len(nonceInput) && i < len(nonce); i++ {
		nonce[i] = nonceInput[i]
	}
	return nonce
}

func (input *TxInput) GetCreateTimeNanos() int64 {
	return (time.Second * time.Duration(input.CreateTime)).Nanoseconds()
}
