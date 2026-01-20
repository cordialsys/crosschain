package crosschain

import (
	"encoding/json"
	"reflect"

	"github.com/cordialsys/crosschain/call"
)

// TxInput is input data to a tx. Depending on the blockchain it can include nonce, recent block hash, account id, ...
type TxInput interface {
	GetDriver() Driver
	TxInputConflicts
	TxInputGasFeeMultiplier
	TxInputGetMaxPossibleFee
}

// For chains/transactions that can benefit from knowing the timestamp
type TxInputWithUnix interface {
	SetUnix(int64)
}

type TxInputGasFeeMultiplier interface {
	SetGasFeePriority(priority GasFeePriority) error
}

type TxInputGetMaxPossibleFee interface {
	// Get the maximum possible fee that could occur for this transaction.
	// This is used to guard against "griefing" attacks where a user is charged way more than they intended.
	// The contract address may be "" when the fee is for the native asset, as is often the case..
	//
	// Note: The caller/user should check this after TxInput has been populated with all other fields, as they can influence
	// what the ultimate max fee is.
	GetFeeLimit() (AmountBlockchain, ContractAddress)
}

// This interface determines whether if different tx inputs conflict with one another.
type TxInputConflicts interface {
	// Test independence of two tx-inputs, assuming the same address is used.
	// Examples:
	// - using the same nonce or sequence is NOT independent
	// - spending the same resources or utxo's is NOT independent
	// - solana (using recent_block_hash) is pretty much always independent
	// This is used to determine if a transaction needs to be queued or if it can be immediately signed & broadcasted.
	IndependentOf(other TxInput) (independent bool)

	// Test if tx-input could possibly result in a "double-send" given the history of past attempts.
	// A double send is a user re-signing their transaction (to overcome a timeout or use new fees), but then risk multiple transactions
	// landing on chain.  A valid re-sign should only occur if it's only possible for one transaction to land out of the total set of attempts.
	// Examples:
	// - Solana typically has no conflicts, but need to ensure (a) new blockhash is used, and (b) sufficient time has passed
	//   to be sure a double send won't occur (return `true`).
	// - If tx-inputs are not independent (spending same resources), then typically double-sends are impossible (and should return `true` here).
	// - If there exists tx-inputs that are fully independent (and not timed out), then a double-send is possible and this should return false.
	SafeFromDoubleSend(previousAttempts TxInput) (safe bool)
}

func IsTypeOf(input TxInput, validTypes ...any) bool {
	if input == nil {
		return false
	}
	for _, validType := range validTypes {
		if reflect.TypeOf(input) == reflect.TypeOf(validType) {
			return true
		}
	}
	return false
}

type TxInputEnvelope struct {
	Type Driver `json:"type"`
}

func NewTxInputEnvelope(envType Driver) *TxInputEnvelope {
	return &TxInputEnvelope{
		Type: envType,
	}
}

type TxVariantInput interface {
	TxInput
	GetVariant() TxVariantInputType
}

// Markers for each type of Variant Tx
type MultiTransferInput interface {
	TxVariantInput
	MultiTransfer()
}
type StakeTxInput interface {
	TxVariantInput
	Staking()
}
type UnstakeTxInput interface {
	TxVariantInput
	Unstaking()
}
type WithdrawTxInput interface {
	TxVariantInput
	Withdrawing()
}
type CallTxInput interface {
	TxVariantInput
	Calling()
}

// TxStatus is the status of a tx on chain, currently success or failure.
type TxStatus uint8

// TxStatus values
const (
	TxStatusSuccess TxStatus = 0
	TxStatusFailure TxStatus = 1
)

// TxHash is a tx hash or id
type TxHash string

// TxSignature is a tx signature
type TxSignature []byte

// NewTxSignatures creates a new array of TxSignature, useful to cast [][]byte into []TxSignature
func NewTxSignatures(data [][]byte) []TxSignature {
	ret := make([]TxSignature, len(data))
	for i, sig := range data {
		ret[i] = TxSignature(sig)
	}
	return ret
}

type SignatureRequest struct {
	// Required: The payload to sign
	Payload []byte
	// May be optionally set, if different from the from-address.
	Signer Address
}

func NewSignatureRequest(payload []byte, signerMaybe ...Address) *SignatureRequest {
	signer := Address("")
	if len(signerMaybe) > 0 {
		signer = signerMaybe[0]
	}
	return &SignatureRequest{
		Payload: payload,
		Signer:  signer,
	}
}

type SignatureResponse struct {
	// Signaure of the payload
	Signature TxSignature
	// Address + public key of the signer
	PublicKey []byte
	Address   Address
}

// Tx is a transaction
type Tx interface {
	Hash() TxHash
	Sighashes() ([]*SignatureRequest, error)
	SetSignatures(...*SignatureResponse) error
	Serialize() ([]byte, error)
}

type TxWithMetadata interface {
	// If the client SubmitTx needs additional metadata, this can be used to define it.
	GetMetadata() ([]byte, bool, error)
}

type TxLegacyGetSignatures interface {
	// Replaced by TxWithMetadata.GetMetadata()
	GetSignatures() []TxSignature
}

type TxAdditionalSighashes interface {
	// This is available in case a transaction needs to make signatures-over-signatures.
	// This should return any _remaining_ signatures requests left to fill.
	// The caller will always call .SetSignatures() after this with all of the signature made so far.
	AdditionalSighashes() ([]*SignatureRequest, error)
}

type TxCall interface {
	Tx
	// Set transaction input.  This may not be needed, but could be used to adjust:
	// * fees
	// * other dynamic chain information not included in the origin Call message
	SetInput(input CallTxInput) error

	// List of addresses that may be needed to sign
	SigningAddresses() []Address

	// List of 3rd party contract addresses that this Call resource interacts with
	// (omit native system contracts)
	ContractAddresses() []ContractAddress

	// Get original serialized message that Call was constructed with
	GetMsg() json.RawMessage

	// Get the method of the call
	GetMethod() call.Method
}
