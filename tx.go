package crosschain

import "encoding/base64"

// TxInput is input data to a tx. Depending on the blockchain it can include nonce, recent block hash, account id, ...
type TxInput interface {
}

// TxInputWithPublicKey is input data to a tx for chains that need to explicitly set the public key, e.g. Cosmos
type TxInputWithPublicKey interface {
	TxInput
	SetPublicKey(PublicKey) error
	SetPublicKeyFromStr(string) error
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

// TxInfoEndpoint is a unified view of an endpoint (source or destination) in a TxInfo.
type TxInfoEndpoint struct {
	Address         Address          `json:"address"`
	ContractAddress ContractAddress  `json:"contract,omitempty"`
	Amount          AmountBlockchain `json:"amount"`
	NativeAsset     NativeAsset      `json:"chain"`
	Asset           Asset            `json:"asset,omitempty"`
	AssetConfig     *AssetConfig     `json:"asset_config,omitempty"`
}

// TxInfo is a unified view of common tx info across multiple blockchains. Use it as an example to build your own.
type TxInfo struct {
	BlockHash       string            `json:"block_hash"`
	TxID            string            `json:"tx_id"`
	ExplorerURL     string            `json:"explorer_url"`
	From            Address           `json:"from"`
	To              Address           `json:"to"`
	ToAlt           Address           `json:"to_alt,omitempty"`
	ContractAddress ContractAddress   `json:"contract,omitempty"`
	Amount          AmountBlockchain  `json:"amount"`
	Fee             AmountBlockchain  `json:"fee"`
	BlockIndex      int64             `json:"block_index,omitempty"`
	BlockTime       int64             `json:"block_time,omitempty"`
	Confirmations   int64             `json:"confirmations,omitempty"`
	Status          TxStatus          `json:"status"`
	Sources         []*TxInfoEndpoint `json:"sources,omitempty"`
	Destinations    []*TxInfoEndpoint `json:"destinations,omitempty"`
	Time            int64             `json:"time,omitempty"`
	TimeReceived    int64             `json:"time_received,omitempty"`
	// If this transaction failed, this is the reason why.
	Error string `json:"error,omitempty"`
}

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
