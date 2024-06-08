package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/golang/protobuf/proto"
	core "github.com/okx/go-wallet-sdk/coins/tron/pb"
)

// Tx for Template
type Tx struct {
	tronTx *core.Transaction
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	hashBase, _ := proto.Marshal(tx.tronTx.RawData)
	digest := sha256.Sum256(hashBase)

	return xc.TxHash(hex.EncodeToString(digest[:]))
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

func (tx *Tx) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	if tx.tronTx != nil {
		for _, sig := range tx.tronTx.Signature {
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return proto.Marshal(tx.tronTx)
}
