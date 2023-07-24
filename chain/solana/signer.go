package solana

import (
	"crypto/ed25519"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/jumpcrypto/crosschain"
)

// Signer for Solana
type Signer struct {
}

// NewSigner creates a new Solana Signer
func NewSigner(cfgI xc.ITask) (xc.Signer, error) {
	return Signer{}, nil
}

// ImportPrivateKey imports a Solana private key
func (signer Signer) ImportPrivateKey(privateKey string) (xc.PrivateKey, error) {
	return xc.PrivateKey(base58.Decode(privateKey)), nil
}

// Sign a Solana tx
func (signer Signer) Sign(privateKey xc.PrivateKey, data xc.TxDataToSign) (xc.TxSignature, error) {
	signatureRaw := ed25519.Sign(ed25519.PrivateKey(privateKey), []byte(data))
	return xc.TxSignature(signatureRaw), nil
}
func (signer Signer) PublicKey(privateKeyBz xc.PrivateKey) (xc.PublicKey, error) {
	privateKey := ed25519.PrivateKey(privateKeyBz)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return xc.PublicKey(publicKey), nil
}
