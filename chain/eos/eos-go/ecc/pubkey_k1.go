package ecc

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

type innerK1PublicKey struct {
}

func newInnerK1PublicKey() innerPublicKey {
	return &innerK1PublicKey{}
}

func (p *innerK1PublicKey) key(content []byte) (*btcec.PublicKey, error) {
	key, err := btcec.ParsePubKey(content)
	if err != nil {
		return nil, fmt.Errorf("parsePubKey: %w", err)
	}

	return key, nil
}

func (p *innerK1PublicKey) prefix() string {
	return PublicKeyPrefixCompat
}

func (p *innerK1PublicKey) keyMaterialSize() *int {
	return publicKeyDataSize
}
