package tx

import (
	"errors"

	xc "github.com/cordialsys/crosschain"
)

// Tx for Template
type Tx struct {
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	return xc.TxHash("not implemented")
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	return []*xc.SignatureRequest{}, errors.New("not implemented")

}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(...*xc.SignatureResponse) error {
	return errors.New("not implemented")
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	return []xc.TxSignature{}
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return []byte{}, errors.New("not implemented")
}
