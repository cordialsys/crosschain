package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/tron/core"
	"github.com/golang/protobuf/proto"
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
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	rawData, err := proto.Marshal(tx.tronTx.GetRawData())
	if err != nil {
		return nil, errors.New("unable to get raw data")
	}

	hasher := sha256.New()
	hasher.Write(rawData)

	return []*xc.SignatureRequest{xc.NewSignatureRequest(hasher.Sum(nil))}, nil

}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	for _, sig := range signatures {
		tx.tronTx.Signature = append(tx.tronTx.Signature, sig.Signature)
	}
	return nil
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return proto.Marshal(tx.tronTx)
}
