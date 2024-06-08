package crosschain

import (
	"encoding/base64"
)

// TxInput is input data to a tx. Depending on the blockchain it can include nonce, recent block hash, account id, ...
type TxInput interface {
	TxInputIsConflict
	TxInputCanRetry
}

// TxInputWithPublicKey is input data to a tx for chains that need to explicitly set the public key, e.g. Cosmos
type TxInputWithPublicKey interface {
	TxInput
	SetPublicKey(PublicKey) error
	SetPublicKeyFromStr(string) error
}

// TxInputWithAmount for chains that can optimize the tx input if they know the amount being transferred.
type TxInputWithAmount interface {
	SetAmount(AmountBlockchain)
}

// For chains/transactions that leverage memo field
type TxInputWithMemo interface {
	SetMemo(string)
}

// For chains/transactions that can benefit from knowing the timestamp
type TxInputWithUnix interface {
	SetUnix(int64)
}

// Implementing this interface determines whether if two different
// input conflict with one another, assuming the same address is used.
// Examples:
// - using the same nonce or sequence is a conflict
// - spending the same resources or utxo's is a conflict
// - solana (using recent_block_hash) doesn't have any conflicts typically
// This is used to determine if a transaction needs to be queued or if it can be immediately signed & broadcasted.
type TxInputIsConflict interface {
	IsConflict(other TxInput) bool
}

// Similar to TxInputCheckConflict, this provides another requirement to check to see if a transaction can be retried
// safely, without risk of double-sending.  This is not used in queuing, only to check for retry safety.
// Examples:
// - Solana typically has no conflicts, but need to ensure (a) new blockhash is used, and (b) sufficient time has passed.
// - If there are conflict(s), then !IsConflict() could be used (a new transaction with a conflict will not double spend).
type TxInputCanRetry interface {
	CanRetry(other TxInput) bool
}

// Legacy
type TxInputWithPricing interface {
	SetUsdPrice(nativeAsset NativeAsset, contract string, priceUsd AmountHumanReadable)
	GetUsdPrice(nativeAsset NativeAsset, contract string) (AmountHumanReadable, bool)
}

type TxInputEnvelope struct {
	Type Driver `json:"type"`
}

func NewTxInputEnvelope(envType Driver) *TxInputEnvelope {
	return &TxInputEnvelope{
		Type: envType,
	}
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

// TxDataToSign is the payload that Signer needs to sign, when "signing a tx". It's sometimes called a sighash.
type TxDataToSign []byte

func (data TxDataToSign) String() string {
	return base64.RawURLEncoding.EncodeToString(data)
}

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

// Tx is a transaction
type Tx interface {
	Hash() TxHash
	Sighashes() ([]TxDataToSign, error)
	AddSignatures(...TxSignature) error
	Serialize() ([]byte, error)
}
