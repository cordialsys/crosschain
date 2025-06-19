package ecc

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

type innerR1PublicKey struct {
}

func newInnerR1PublicKey() innerPublicKey {
	return &innerR1PublicKey{}
}

func (p *innerR1PublicKey) key(content []byte) (*btcec.PublicKey, error) {
	key, err := btcec.ParsePubKey(content)
	if err != nil {
		return nil, fmt.Errorf("parsePubKey: %w", err)
	}

	return key, nil
}

func (p *innerR1PublicKey) prefix() string {
	return PublicKeyR1Prefix
}

func (p *innerR1PublicKey) keyMaterialSize() *int {
	return publicKeyDataSize
}
