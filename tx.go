package crosschain

import (
	"encoding/base64"
)

// TxInput is input data to a tx. Depending on the blockchain it can include nonce, recent block hash, account id, ...
type TxInput interface {
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
