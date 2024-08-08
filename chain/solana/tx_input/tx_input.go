package tx_input

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

// TxInput for Solana
type TxInput struct {
	xc.TxInputEnvelope
	RecentBlockHash     solana.Hash         `json:"recent_block_hash,omitempty"`
	ToIsATA             bool                `json:"to_is_ata,omitempty"`
	TokenProgram        solana.PublicKey    `json:"token_program"`
	ShouldCreateATA     bool                `json:"should_create_ata,omitempty"`
	SourceTokenAccounts []*TokenAccount     `json:"source_token_accounts,omitempty"`
	PrioritizationFee   xc.AmountBlockchain `json:"prioritization_fee,omitempty"`
	Timestamp           int64               `json:"timestamp,omitempty"`
}

type TokenAccount struct {
	Account solana.PublicKey    `json:"account,omitempty"`
	Balance xc.AmountBlockchain `json:"balance,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverSolana
}

// Solana recent-block-hash timeout margin
const SafetyTimeoutMargin = (5 * time.Minute)

// Returns the microlamports to set the compute budget unit price.
// It will not go about the max price amount for safety concerns.
func (input *TxInput) GetLimitedPrioritizationFee(chain *xc.ChainConfig) uint64 {
	fee := input.PrioritizationFee.Uint64()
	max := uint64(chain.ChainMaxGasPrice)
	if max == 0 {
		// set default max price to spend max 1 SOL on a transaction
		// 1 SOL = (1 * 10 ** 9) * 10 ** 6 microlamports
		// /200_000 compute units
		max = 5_000_000_000
	}
	if fee > max {
		fee = max
	}
	return fee
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	multipliedFee := multiplier.Mul(decimal.NewFromBigInt(input.PrioritizationFee.Int(), 0)).BigInt()
	input.PrioritizationFee = xc.AmountBlockchain(*multipliedFee)
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// no conflicts on solana as txs are easily parallelizeable through
	// the recent-block-hash mechanism.
	return true
}

func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	for _, other := range others {
		oldInput, ok := other.(*TxInput)
		if ok {
			diff := input.Timestamp - oldInput.Timestamp
			// solana blockhash lasts only ~1 minute -> we'll require a 5 min period
			// and different hash to consider it safe from double-send.
			if diff < int64(SafetyTimeoutMargin.Seconds()) || oldInput.RecentBlockHash.Equals(input.RecentBlockHash) {
				// not yet safe
				return false
			}
		} else {
			// can't tell (this shouldn't happen) - default false
			return false
		}
	}
	// all timed out - we're safe
	return true
}

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
}

// NewTxInput returns a new Solana TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSolana),
	}
}
