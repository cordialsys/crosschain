package solana

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
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
	// try hex first
	bz, err := hex.DecodeString(privateKey)
	if err == nil && len(bz) == 32 {
		key := ed25519.NewKeyFromSeed(bz)
		return xc.PrivateKey(key), nil
	}
	// use base58 directly
	base58bz := base58.Decode(privateKey)
	if len(base58bz) != 64 {
		return nil, errors.New("expected ed25519 key to be 64 or 32 bytes")
	}
	return xc.PrivateKey(base58bz), nil
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
