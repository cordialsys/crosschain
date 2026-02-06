package tx

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/btcutil/bech32"
	"golang.org/x/crypto/blake2b"
	"google.golang.org/protobuf/proto"
)

// Tx for EGLD (MultiversX)
type Tx struct {
	Nonce     uint64 `json:"nonce"`
	Value     string `json:"value"`
	Receiver  string `json:"receiver"`
	Sender    string `json:"sender"`
	GasPrice  uint64 `json:"gasPrice"`
	GasLimit  uint64 `json:"gasLimit"`
	Data      []byte `json:"data,omitempty"`
	ChainID   string `json:"chainID"`
	Version   uint32 `json:"version"`
	Options   uint32 `json:"options,omitempty"`
	Guardian  string `json:"guardian,omitempty"`
	Signature string `json:"signature,omitempty"`
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
// For MultiversX, the hash is calculated as Blake2b hash of the protobuf-serialized transaction with signature included
func (tx Tx) Hash() xc.TxHash {
	if tx.Signature == "" {
		return xc.TxHash("")
	}

	protoTx, err := tx.toProto(true)
	if err != nil {
		return xc.TxHash("")
	}

	protoBytes, err := proto.Marshal(protoTx)
	if err != nil {
		return xc.TxHash("")
	}

	hash := blake2b.Sum256(protoBytes)
	return xc.TxHash(hex.EncodeToString(hash[:]))
}

// toProto converts the Tx to protobuf Transaction
func (tx *Tx) toProto(includeSignature bool) (*Transaction, error) {
	value, ok := new(big.Int).SetString(tx.Value, 10)
	if !ok {
		return nil, errors.New("invalid value")
	}

	// MultiversX uses custom big integer encoding: [0x00, 0x00] for zero, [0x00, ...bytes] for non-zero
	var valueBytes []byte
	if value.Sign() == 0 {
		valueBytes = []byte{0x00, 0x00}
	} else {
		valueBytes = append([]byte{0x00}, value.Bytes()...)
	}

	receiverBytes, err := decodeBech32Address(tx.Receiver)
	if err != nil {
		return nil, err
	}

	senderBytes, err := decodeBech32Address(tx.Sender)
	if err != nil {
		return nil, err
	}

	var guardianBytes []byte
	if tx.Guardian != "" {
		guardianBytes, err = decodeBech32Address(tx.Guardian)
		if err != nil {
			return nil, err
		}
	}

	data := tx.Data
	if data == nil {
		data = []byte{}
	}

	protoTx := &Transaction{
		Nonce:       tx.Nonce,
		Value:       valueBytes,
		RcvAddr:     receiverBytes,
		RcvUserName: []byte{},
		SndAddr:     senderBytes,
		SndUserName: []byte{},
		GasPrice:    tx.GasPrice,
		GasLimit:    tx.GasLimit,
		Data:        data,
		ChainID:     []byte(tx.ChainID),
		Version:     tx.Version,
	}

	if tx.Options != 0 {
		protoTx.Options = tx.Options
	}

	if len(guardianBytes) > 0 {
		protoTx.GuardAddr = guardianBytes
	}

	if includeSignature && tx.Signature != "" {
		sigBytes, err := hex.DecodeString(tx.Signature)
		if err != nil {
			return nil, err
		}
		protoTx.Signature = sigBytes
	}

	return protoTx, nil
}

// decodeBech32Address decodes a bech32 address to bytes
func decodeBech32Address(address string) ([]byte, error) {
	// Import the bech32 decoder
	_, decoded, err := bech32.DecodeToBase256(address)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// Sighashes returns the tx payload to sign, aka sighash
// MultiversX signs the JSON serialization of the transaction
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	jsonBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}

	return []*xc.SignatureRequest{
		xc.NewSignatureRequest(jsonBytes),
	}, nil
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) == 0 {
		return errors.New("no signatures provided")
	}

	if len(signatures[0].Signature) != 64 {
		return errors.New("invalid signature length: expected 64 bytes")
	}

	// Convert signature bytes to hex string
	tx.Signature = hex.EncodeToString(signatures[0].Signature)

	return nil
}

// Serialize returns the serialized tx
// Returns JSON representation of the transaction including the signature
func (tx Tx) Serialize() ([]byte, error) {
	if tx.Signature == "" {
		return nil, errors.New("transaction not signed")
	}

	return json.Marshal(tx)
}
