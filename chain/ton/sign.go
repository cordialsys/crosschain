package ton

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type Signer struct {
}

var _ xc.Signer = Signer{}

func NewSigner(cfgI xc.ITask) (xc.Signer, error) {
	return Signer{}, nil
}

func (signer Signer) ImportPrivateKey(privateKey string) (xc.PrivateKey, error) {
	// try hex first
	bz, err := hex.DecodeString(privateKey)
	if err != nil {
		return nil, err
	}
	if len(bz) != ed25519.SeedSize {
		return nil, fmt.Errorf("expected private key seed to be %d bytes, got %d bytes", ed25519.SeedSize, len(bz))
	}
	key := ed25519.NewKeyFromSeed(bz)
	return xc.PrivateKey(key), nil
}

func (signer Signer) Sign(privateKey xc.PrivateKey, data xc.TxDataToSign) (xc.TxSignature, error) {
	signatureRaw := ed25519.Sign(ed25519.PrivateKey(privateKey), []byte(data))
	return xc.TxSignature(signatureRaw), nil
}
func (signer Signer) PublicKey(privateKeyBz xc.PrivateKey) (xc.PublicKey, error) {
	privateKey := ed25519.PrivateKey(privateKeyBz)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return xc.PublicKey(publicKey), nil
}
