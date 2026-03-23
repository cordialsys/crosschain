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
	RecentBlockHash     solana.Hash      `json:"recent_block_hash,omitempty"`
	ToIsATA             bool             `json:"to_is_ata,omitempty"`
	TokenProgram        solana.PublicKey `json:"token_program"`
	ShouldCreateATA     bool             `json:"should_create_ata,omitempty"`
	SourceTokenAccounts []*TokenAccount  `json:"source_token_accounts,omitempty"`
	// This is in "microlamports"
	// https://solana.com/docs/core/fees#compute-units-and-limits
	PrioritizationFee xc.AmountBlockchain `json:"prioritization_fee,omitempty"`
	Timestamp         int64               `json:"timestamp,omitempty"`
	// The base fee is applied for every signature on the transaction
	BaseFee xc.AmountBlockchain `json:"base_fee,omitempty"`
	// The estimated compute units used by the transaction (basically the gas usage)
	UnitsConsumed uint64 `json:"units_consumed,omitempty"`

	// Durable nonce fields -- when set, the transaction uses a durable nonce instead of a recent blockhash.
	// The nonce value stored in the nonce account, used as the transaction's "recent blockhash".
	DurableNonce solana.Hash `json:"durable_nonce,omitempty"`
	// The on-chain nonce account address.
	DurableNonceAccount solana.PublicKey `json:"durable_nonce_account,omitempty"`
	// If true, the nonce account needs to be created and initialized before use.
	ShouldCreateDurableNonce bool `json:"should_create_durable_nonce,omitempty"`
}
type GetTxInfo interface {
	GetTimestamp() int64
	GetRecentBlockhash() solana.Hash
}

// GetDurableNonceInfo is an interface to retrieve durable nonce information from a TxInput.
type GetDurableNonceInfo interface {
	GetDurableNonceAccount() solana.PublicKey
	GetDurableNonceValue() solana.Hash
	IsDurableNonceEnabled() bool
}

type TokenAccount struct {
	Account solana.PublicKey    `json:"account,omitempty"`
	Balance xc.AmountBlockchain `json:"balance,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ GetTxInfo = &TxInput{}
var _ GetDurableNonceInfo = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
}

func (input *TxInput) GetTimestamp() int64 {
	return input.Timestamp
}

func (input *TxInput) GetRecentBlockhash() solana.Hash {
	return input.RecentBlockHash
}

// HasDurableNonce returns true if the transaction should use an existing durable nonce.
// Returns false when the nonce account needs to be created (ShouldCreateDurableNonce=true).
func (input *TxInput) HasDurableNonce() bool {
	return !input.DurableNonceAccount.IsZero() && !input.ShouldCreateDurableNonce
}

func (input *TxInput) GetDurableNonceAccount() solana.PublicKey {
	return input.DurableNonceAccount
}

func (input *TxInput) GetDurableNonceValue() solana.Hash {
	return input.DurableNonce
}

func (input *TxInput) IsDurableNonceEnabled() bool {
	return input.HasDurableNonce()
}

// GetBlockhashForTx returns the blockhash to use for the transaction.
// If a durable nonce is set and initialized, the nonce value is used instead of a recent blockhash.
// When the nonce account needs to be created first, the recent blockhash is used.
func (input *TxInput) GetBlockhashForTx() solana.Hash {
	if input.HasDurableNonce() {
		return input.DurableNonce
	}
	return input.RecentBlockHash
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
	maxSpendMicroLamports := gasLimit.Mul(&input.PrioritizationFee)
	tenPow6 := xc.NewAmountBlockchainFromUint64(1_000_000)
	maxSpend := maxSpendMicroLamports.Div(&tenPow6)

	// calculate the base fee (# of signatures * base fee)
	feePerSignature := input.BaseFee
	numSignatures := xc.NewAmountBlockchainFromUint64(1)
	totalBaseFee := feePerSignature.Mul(&numSignatures)

	// prioritization + base fees
	maxSpend = maxSpend.Add(&totalBaseFee)
	return maxSpend, ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return true
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if otherNonce, ok := other.(GetDurableNonceInfo); ok {
		// With durable nonces, two transactions conflict only if they use the same nonce value.
		// Same nonce account + same nonce value = NOT independent (only one can succeed).
		// Same nonce account + different nonce value = independent (each uses its own nonce).
		if input.HasDurableNonce() && otherNonce.IsDurableNonceEnabled() {
			if input.DurableNonceAccount.Equals(otherNonce.GetDurableNonceAccount()) {
				return !input.DurableNonce.Equals(otherNonce.GetDurableNonceValue())
			}
		}
	}
	// no conflicts on solana as txs are easily parallelizeable through
	// the recent-block-hash mechanism.
	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	// When using durable nonces: the nonce can only be consumed once.
	// Same nonce value = SAFE (only one transaction can land, the runtime rejects duplicates).
	// Different nonce values = NOT SAFE (both could land, causing a double-send).
	if otherNonce, ok := other.(GetDurableNonceInfo); ok {
		if input.HasDurableNonce() && otherNonce.IsDurableNonceEnabled() {
			if input.DurableNonceAccount.Equals(otherNonce.GetDurableNonceAccount()) {
				// Same nonce account + same nonce value = only one tx can succeed
				return input.DurableNonce.Equals(otherNonce.GetDurableNonceValue())
			}
			// Different nonce accounts: both could land, check normal timeout
		}
	}

	// For recent blockhash (non-durable-nonce) transactions
	oldInput, ok := other.(GetTxInfo)
	if !ok {
		return false
	}
	diff := input.Timestamp - oldInput.GetTimestamp()
	// solana blockhash lasts only ~1 minute -> we'll require a 5 min period
	// and different hash to consider it safe from double-send.
	if diff < int64(SafetyTimeoutMargin.Seconds()) || oldInput.GetRecentBlockhash().Equals(input.GetRecentBlockhash()) {
		// not yet safe
		return false
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
