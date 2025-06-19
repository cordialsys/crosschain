package ecc

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

type innerWAPublicKey struct {
}

func newInnerWAPublicKey() innerPublicKey {
	return &innerWAPublicKey{}
}

func (p *innerWAPublicKey) key(content []byte) (*btcec.PublicKey, error) {
	// // First byte can be either 0x02 or 0x03
	// if content[0] != 0x02 && content[0] != 0x03 {
	// 	return nil, fmt.Errorf("expected compressed public key format, expecting 0x02 or 0x03, got %d", content[0])
	// }

	// ySign := content[0] & 0x01
	// x := content[1:33]

	// X := new(big.Int).SetBytes(x)
	// Y, err := decompressPoint(X, ySign == 1)
	// if err != nil {
	// 	return nil, fmt.Errorf("unable to decompress compressed publick key material: %w", err)
	// }
	// btcec.NewPublicKey(X, Y)

	// return &btcec.PublicKey{
	// 	Curve: elliptic.P256(),
	// 	X:     X,
	// 	Y:     Y,
	// }, nil
	return nil, fmt.Errorf("not implemented")
}

func (p *innerWAPublicKey) keyMaterialSize() *int {
	return nil
}

func (p *innerWAPublicKey) prefix() string {
	return PublicKeyWAPrefix
}
