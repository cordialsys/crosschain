package ecc

import (
	"github.com/btcsuite/btcd/btcutil/base58"
)

type innerR1Signature struct {
}

func newInnerR1Signature() innerSignature {
	return &innerR1Signature{}
}

func (s innerR1Signature) string(content []byte) string {
	checksum := ripemd160checksumHashCurve(content, CurveR1)
	buf := append(content[:], checksum...)
	return "SIG_R1_" + base58.Encode(buf)
}

func (s innerR1Signature) signatureMaterialSize() *int {
	return signatureDataSize
}
