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
	// The base fee is applied for every signature on the transaction
	BaseFee xc.AmountBlockchain `json:"base_fee,omitempty"`
	// The estimated compute units used by the transaction (basically the gas usage)
	UnitsConsumed uint64 `json:"units_consumed,omitempty"`
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
func (input *TxInput) GetPrioritizationFee() uint64 {
	fee := input.PrioritizationFee.Uint64()
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

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// https://solana.com/docs/core/fees#key-points
	var computeUnits uint64
	if input.UnitsConsumed == 0 && input.PrioritizationFee.Uint64() > 0 {
		// assume the worst case scenario if there's no estimated compute usage
		// https://solana.com/docs/core/fees#compute-units-and-limits
		computeUnits = 1_400_000
	} else {
		computeUnits = input.UnitsConsumed
	}

	// calculate the max spend for the tx: (compute units * priority fee)
	gasLimit := xc.NewAmountBlockchainFromUint64(computeUnits)
	maxSpend := gasLimit.Mul(&input.PrioritizationFee)

	// calculate the base fee (# of signatures * base fee)
	feePerSignature := input.BaseFee
	numSignatures := xc.NewAmountBlockchainFromUint64(1)
	totalBaseFee := feePerSignature.Mul(&numSignatures)

	// prioritization + base fees
	maxSpend = maxSpend.Add(&totalBaseFee)
	return maxSpend, ""
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
