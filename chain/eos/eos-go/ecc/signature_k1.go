package ecc

import (
	"github.com/btcsuite/btcd/btcutil/base58"
)

type innerK1Signature struct {
}

func newInnerK1Signature() innerSignature {
	return &innerK1Signature{}
}

func (s innerK1Signature) string(content []byte) string {
	checksum := ripemd160checksumHashCurve(content, CurveK1)
	buf := append(content[:], checksum...)
	return "SIG_K1_" + base58.Encode(buf)
}

func (s innerK1Signature) signatureMaterialSize() *int {
	return signatureDataSize
}
