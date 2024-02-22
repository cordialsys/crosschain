package tron

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/crypto"
)

// Signer for Tron
type Signer struct {
}

// NewSigner creates a new Tron Signer
func NewSigner(asset xc.ITask) (xc.Signer, error) {
	return Signer{}, nil
}

var _ xc.Signer = &Signer{}

// ImportPrivateKey imports a Tron private key
func (signer Signer) ImportPrivateKey(privateKey string) (xc.PrivateKey, error) {
	bytesPri, err := hex.DecodeString(privateKey)
	return xc.PrivateKey(bytesPri), err
}

// Sign an EVM tx
func (signer Signer) Sign(privateKey xc.PrivateKey, data xc.TxDataToSign) (xc.TxSignature, error) {
	ecdsaKey, err := crypto.HexToECDSA(hex.EncodeToString(privateKey))
	if err != nil {
		return []byte{}, err
	}
	signatureRaw, err := crypto.Sign(data, ecdsaKey)
	return xc.TxSignature(signatureRaw), err
}

func (signer Signer) PublicKey(privateKey xc.PrivateKey) (xc.PublicKey, error) {
	ecdsaKey, err := crypto.HexToECDSA(hex.EncodeToString(privateKey))
	if err != nil {
		return []byte{}, err
	}
	pubkey := crypto.FromECDSAPub(&ecdsaKey.PublicKey)
	return pubkey, nil
}
