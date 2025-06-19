package ecc

import (
	"github.com/btcsuite/btcd/btcutil/base58"
)

type innerWASignature struct {
}

func newInnerWASignature() innerSignature {
	return &innerWASignature{}
}

func (s innerWASignature) string(content []byte) string {
	checksum := ripemd160checksumHashCurve(content, CurveWA)
	buf := append(content[:], checksum...)
	return "SIG_WA_" + base58.Encode(buf)
}

func (s innerWASignature) signatureMaterialSize() *int {
	return nil
}
