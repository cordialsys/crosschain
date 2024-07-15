package substrate

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
)

// Signer for Substrate
type Signer struct {
}

// NewSigner creates a new Substrate Signer
func NewSigner(cfgI xc.ITask) (xc.Signer, error) {
	return Signer{}, nil
}

// ImportPrivateKey imports a Substrate private key
func (signer Signer) ImportPrivateKey(privateKey string) (xc.PrivateKey, error) {
	// Only allow the raw seed, not the mnemonic, to allow more reasonable conversion to bytearray.
	if strings.Contains(privateKey, " ") {
		return []byte{}, errors.New("only raw seed is supported, not mnemonic")
	}
	seedBytes, err := codec.HexDecodeString(privateKey)
	if err != nil {
		return []byte{}, err
	}
	if len(seedBytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("expected private key seed to be %d bytes, got %d bytes", ed25519.SeedSize, len(seedBytes))
	}
	return xc.PrivateKey(seedBytes), nil

}

// Sign a Substrate tx
// The binary data must be from codec.Encode(payload), where payload is types.ExtrinsicPayload
func (signer Signer) Sign(privateKey xc.PrivateKey, payload xc.TxDataToSign) (xc.TxSignature, error) {
	return signature.Sign(payload, hex.EncodeToString(privateKey))
}
func (signer Signer) PublicKey(privateKeyBz xc.PrivateKey) (xc.PublicKey, error) {
	privateKey := ed25519.NewKeyFromSeed(privateKeyBz)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return xc.PublicKey(publicKey), nil
}
