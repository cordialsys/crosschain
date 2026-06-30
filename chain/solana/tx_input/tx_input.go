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
	// The authority/base account for the durable nonce account. Omitted in older
	// inputs, which should continue to use the transaction's from address.
	DurableNonceAuthority solana.PublicKey `json:"durable_nonce_authority,omitempty"`
	// If true, the nonce account needs to be created and initialized before use.
	ShouldCreateDurableNonce bool `json:"should_create_durable_nonce,omitempty"`

	// Fee-payer durable nonce fields are separate from the legacy durable_nonce
	// fields so older builders keep using the main address nonce account.
	FeePayerDurableNonce          solana.Hash         `json:"fee_payer_durable_nonce,omitempty"`
	FeePayerDurableNonceAccount   solana.PublicKey    `json:"fee_payer_durable_nonce_account,omitempty"`
	FeePayerDurableNonceAuthority solana.PublicKey    `json:"fee_payer_durable_nonce_authority,omitempty"`
	ShouldCreateFeePayerNonce     bool                `json:"should_create_fee_payer_durable_nonce,omitempty"`
	FeePayerBaseFee               xc.AmountBlockchain `json:"fee_payer_base_fee,omitempty"`
}
type GetTxInfo interface {
	GetTimestamp() int64
	GetRecentBlockhash() solana.Hash

	GetFromAddressDurableNonceValue() solana.Hash
	HasFromAddressDurableNonce() bool
	EffectiveDurableNonceState() DurableNonceState

	// Check if the transaction is using our durable nonce account.
	// If so, we should try to sync to using our detect nonce value,
	// to ensure smooth conflict resolution.
	DoesTxUseOurDurableNonce(tx *solana.Transaction) (isAccountReferenced bool, nonceValue solana.Hash, nonceOk bool)
}

type TokenAccount struct {
	Account solana.PublicKey    `json:"account,omitempty"`
	Balance xc.AmountBlockchain `json:"balance,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ GetTxInfo = &TxInput{}
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

func (input *TxInput) GetDurableNonceForFromAddress(fromAddress solana.PublicKey) (nonceAuthority solana.PublicKey, nonceAccount solana.PublicKey, nonceValue solana.Hash, needsCreation bool) {
	authority := input.DurableNonceAuthority
	if authority.IsZero() {
		authority = fromAddress
	}
	return authority, input.DurableNonceAccount, input.DurableNonce, input.ShouldCreateDurableNonce
}

func (input *TxInput) GetDurableNonceForFeePayerAddress(feePayerAddress solana.PublicKey) (nonceAuthority solana.PublicKey, nonceAccount solana.PublicKey, nonceValue solana.Hash, needsCreation bool) {
	authority := input.FeePayerDurableNonceAuthority
	if authority.IsZero() {
		authority = feePayerAddress
	}
	return authority, input.FeePayerDurableNonceAccount, input.FeePayerDurableNonce, input.ShouldCreateFeePayerNonce
}

// HasDurableNonce returns true if the transaction should use an existing durable nonce.
// Returns false when the nonce account needs to be created (ShouldCreateDurableNonce=true).
func (input *TxInput) HasFromAddressDurableNonce() bool {
	return !input.DurableNonceAccount.IsZero() && !input.ShouldCreateDurableNonce
}

func (input *TxInput) GetFromAddressDurableNonceAccount() solana.PublicKey {
	return input.DurableNonceAccount
}

func (input *TxInput) GetFromAddressDurableNonceValue() solana.Hash {
	return input.DurableNonce
}

func (input *TxInput) IsCreatingDurableNonceAccount() bool {
	return input.ShouldCreateDurableNonce && !input.DurableNonceAccount.IsZero()
}

func (input *TxInput) HasFeePayerDurableNonce() bool {
	return !input.FeePayerDurableNonceAccount.IsZero() && !input.ShouldCreateFeePayerNonce
}

func (input *TxInput) IsCreatingFeePayerDurableNonceAccount() bool {
	return input.ShouldCreateFeePayerNonce && !input.FeePayerDurableNonceAccount.IsZero()
}

func (input *TxInput) DoesTxUseOurDurableNonce(tx *solana.Transaction) (isAccountReferenced bool, nonceValue solana.Hash, nonceOk bool) {
	referenced := false

	if input.doesTxUseDurableNonce(tx, input.DurableNonceAccount, input.DurableNonce) {
		referenced = true
		if input.HasFromAddressDurableNonce() {
			return true, input.DurableNonce, true
		}
	}

	if input.doesTxUseDurableNonce(tx, input.FeePayerDurableNonceAccount, input.FeePayerDurableNonce) {
		referenced = true
		if input.HasFeePayerDurableNonce() {
			return true, input.FeePayerDurableNonce, true
		}
	}
	return referenced, solana.Hash{}, false
}

func (input *TxInput) doesTxUseDurableNonce(tx *solana.Transaction, account solana.PublicKey, nonce solana.Hash) bool {
	if tx.Message.RecentBlockhash.Equals(nonce) && !nonce.IsZero() {
		return true
	}
	usingDurableNonce := false
	for _, accountKey := range tx.Message.AccountKeys {
		if account.Equals(accountKey) {
			usingDurableNonce = true
			break
		}
	}

	return usingDurableNonce
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverSolana
}

// Solana recent-block-hash timeout margin
const SafetyTimeoutMargin = (10 * time.Minute)

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
	if input.ShouldCreateFeePayerNonce && !input.FeePayerBaseFee.IsZero() {
		// use base-fee for the fee-payer nonce account
		feePerSignature = input.FeePayerBaseFee
	}
	numSignatures := xc.NewAmountBlockchainFromUint64(1)
	totalBaseFee := feePerSignature.Mul(&numSignatures)

	// prioritization + base fees
	maxSpend = maxSpend.Add(&totalBaseFee)
	return maxSpend, ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return true
}

func (input *TxInput) MatchDurableNonce(otherNonce GetTxInfo) (accountMatch, nonceMatch bool) {
	mine := input.EffectiveDurableNonceState()
	otherState := otherNonce.EffectiveDurableNonceState()
	if mine.account.IsZero() || otherState.account.IsZero() || !mine.account.Equals(otherState.account) {
		return false, false
	}
	// Both creating the same nonce account = conflict
	if mine.creating && otherState.creating {
		return true, true
	}
	// Both using the same nonce value = conflict (only one can succeed)
	// Different nonce values = independent (each uses its own nonce)
	if mine.has && otherState.has {
		return true, mine.nonce.Equals(otherState.nonce)
	}
	return true, false
}

type DurableNonceState struct {
	account  solana.PublicKey
	nonce    solana.Hash
	has      bool
	creating bool
}

func (input *TxInput) EffectiveDurableNonceState() DurableNonceState {
	// consider fee-payer first as it is prioritized by the transaction builder
	feePayerState := DurableNonceState{
		account:  input.FeePayerDurableNonceAccount,
		nonce:    input.FeePayerDurableNonce,
		has:      input.HasFeePayerDurableNonce(),
		creating: input.IsCreatingFeePayerDurableNonceAccount(),
	}
	if feePayerState.has || feePayerState.creating {
		return feePayerState
	}
	return DurableNonceState{
		account:  input.DurableNonceAccount,
		nonce:    input.DurableNonce,
		has:      input.HasFromAddressDurableNonce(),
		creating: input.IsCreatingDurableNonceAccount(),
	}
}

func (input *TxInput) DidTimeoutOccur(other GetTxInfo) (timeout bool) {
	diff := input.Timestamp - other.GetTimestamp()
	if diff < int64(SafetyTimeoutMargin.Seconds()) || other.GetRecentBlockhash().Equals(input.GetRecentBlockhash()) {
		return false
	}
	return true
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if otherNonce, ok := other.(GetTxInfo); ok {
		// if input.HasDurableNonce() {
		_, sameNonce := input.MatchDurableNonce(otherNonce)
		if sameNonce {
			// one of the transactions will fail
			return false
		} else {
			// both work
			return true
		}
	}
	// solana transactions are always independent if no durable nonce
	return true
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	otherInput, ok := other.(GetTxInfo)
	if !ok {
		return false
	}

	if input.HasFromAddressDurableNonce() || input.HasFeePayerDurableNonce() {
		sameAccount, sameNonce := input.MatchDurableNonce(otherInput)
		if sameAccount {
			if sameNonce {
				// safe
				return true
			} else {
				return false
			}
		}
	}

	// For recent blockhash (non-durable-nonce) transactions
	if input.DidTimeoutOccur(otherInput) {
		return true
	}

	return false
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
