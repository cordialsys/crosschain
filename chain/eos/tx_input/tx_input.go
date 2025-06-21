package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// Expiration period used for transactions.
const ExpirationPeriod = 10 * time.Minute

// The target RAM balance to try to maintain/float on transacting accounts.
// Some transactions may require RAM if they add some new ledger entry (but not always).
// Rather than try to simulate it to figure it out, we just maintain a target RAM balance.
// const TargetRam = 2 * 1024
const TargetRam = 1000

type TxInput struct {
	xc.TxInputEnvelope
	Timestamp int64 `json:"timestamp"`

	ChainID     []byte `json:"chain_id"`
	HeadBlockID []byte `json:"head_block_id"`

	// The account of the address that is sending the transaction.
	FromAccount     string `json:"from_account"`
	FeePayerAccount string `json:"fee_payer_account"`

	// The symbol to use for the asset contract in the transaction
	Symbol string `json:"symbol"`

	// Information used to be able to conditionally buy or sell RAM.
	AvailableRam int64 `json:"available_ram"`
	// In uS
	AvailableCPU int64 `json:"available_cpu"`
	// in bytes
	AvailableNET int64               `json:"available_net"`
	TargetRam    int64               `json:"target_ram"`
	EosBalance   xc.AmountBlockchain `json:"eos_balance"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

func init() {
	// Uncomment this line to register the driver input for serialization/derserialization
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
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
