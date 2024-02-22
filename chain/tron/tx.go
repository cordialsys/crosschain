package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/golang/protobuf/proto"
)

// Tx for Template
type Tx struct {
	tronTx *core.Transaction
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	serialised, err := tx.Serialize()
	if err != nil {
		panic(err)
	}

	hasher := sha256.New()
	hasher.Write(serialised)

	return xc.TxHash(hex.EncodeToString(hasher.Sum(nil)))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	rawData, err := proto.Marshal(tx.tronTx.GetRawData())
	if err != nil {
		return nil, errors.New("unable to get raw data")
	}

	hasher := sha256.New()
	hasher.Write(rawData)

	return []xc.TxDataToSign{hasher.Sum(nil)}, nil

}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	for _, sig := range signatures {
		tx.tronTx.Signature = append(tx.tronTx.Signature, sig)
	}
	return nil
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return proto.Marshal(tx.tronTx.GetRawData())
}
