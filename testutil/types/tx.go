package testutil

import (
	xc "github.com/cordialsys/crosschain"
)

// An object that only supports .Serialize for SubmitTx()
type MockXcTx struct {
	SerializedSignedTx []byte
	Signatures         []xc.TxSignature
}

var _ xc.Tx = &MockXcTx{}

func (tx *MockXcTx) Hash() xc.TxHash {
	panic("not supported")
}
func (tx *MockXcTx) Sighashes() ([]xc.TxDataToSign, error) {
	panic("not supported")
}
func (tx *MockXcTx) AddSignatures(...xc.TxSignature) error {
	panic("not supported")
}
func (tx *MockXcTx) GetSignatures() []xc.TxSignature {
	return tx.Signatures
}
func (tx *MockXcTx) Serialize() ([]byte, error) {
	return tx.SerializedSignedTx, nil
}
