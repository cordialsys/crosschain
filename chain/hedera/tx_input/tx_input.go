package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

const TIME_MARGIN_MULTIPLIER = int64(2)

// TxInput for Hedera
type TxInput struct {
	xc.TxInputEnvelope
	// Sender account id in `0.0.12345` format
	AccountId string `json:"account_id"`
	// Node account id in `0.0.12345` format
	NodeAccountID string `json:"node_account_id"`
	// Valid transaction timestamp in unix nanos
	ValidStartTimestamp int64 `json:"valid_start_timestamp"`
	// Max fee that the transaction can consume
	MaxTransactionFee uint64 `json:"max_transaction_fee"`
	// Transaction is considered outdated after: `ValidStartTimestamp + ValidTime`
	// Max 180 seconds
	ValidTime int64 `json:"valid_time"`
	// Transaction memo, max 100 characters
	Memo string `json:"memo"`
}

var _ xc.TxInput = &TxInput{}
var _ ValidStartGetter = &TxInput{}

type ValidStartGetter interface {
	GetTimestampNano() int64
	GetExpiration() int64
}

func init() {
	// Uncomment this line to register the driver input for serialization/derserialization
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverHedera,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverHedera
}

func (input *TxInput) SetGasFeePriority(priority xc.GasFeePriority) error {
	multiplier, err := priority.GetDefault()
	if err != nil {
		return err
	}
	xcFee := xc.NewAmountBlockchainFromUint64(input.MaxTransactionFee)
	multipliedMaxFee := multiplier.Mul(decimal.Decimal(xcFee.ToHuman(0)))
	input.MaxTransactionFee = multipliedMaxFee.BigInt().Uint64()
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(0), ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	oldInput, ok := other.(ValidStartGetter)
	if ok {
		return input.GetTimestampNano() != oldInput.GetTimestampNano()
	}
	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if other == nil {
		return true
	}
	o, ok := other.(ValidStartGetter)
	if ok {
		return input.GetTimestampNano() > o.GetExpiration()
	} else {
		return false
	}
}

func (input *TxInput) GetTimestampNano() int64 {
	return input.ValidStartTimestamp
}

func (input *TxInput) GetExpiration() int64 {
	ux := time.Unix(0, input.ValidStartTimestamp)
	expirationPeriod := time.Second * time.Duration(input.ValidTime*TIME_MARGIN_MULTIPLIER)
	expiration := ux.Add(expirationPeriod)
	return expiration.UnixNano()
}
