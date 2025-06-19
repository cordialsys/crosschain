package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Timestamp int64 `json:"timestamp"`

	ChainID     []byte `json:"chain_id"`
	HeadBlockID []byte `json:"head_block_id"`

	// The account of the address that is sending the transaction.
	FromAccount string `json:"from_account"`

	// The symbol to use for the asset contract in the transaction
	Symbol string `json:"symbol"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

func init() {
	// Uncomment this line to register the driver input for serialization/derserialization
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverEOS,
		},
	}
}

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
}

const ExpirationPeriod = 10 * time.Minute

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverEOS
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// get the max possible fee that could be spent on this transaction
	return xc.NewAmountBlockchainFromUint64(0), ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if other, ok := other.(*TxInput); ok {
		// Consider independent if the time differece exceeds the expiration period, with a 60s tolerance.
		diff := input.Timestamp - other.Timestamp
		if diff < 0 {
			diff = -diff
		}
		if (int64(ExpirationPeriod.Seconds()) + 60) < (diff) {
			return true
		}
	}

	return
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(other, input) {
		return false
	}
	return !input.IndependentOf(other)
}
