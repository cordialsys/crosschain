package substrate

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"

	sr25519 "github.com/ChainSafe/go-schnorrkel"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	"github.com/gtank/merlin"
)

type Signer struct {
}

func NewSigner(cfgI xc.ITask) (xc.Signer, error) {
	return Signer{}, nil
}

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

func (signer Signer) Sign(privateKeyBz xc.PrivateKey, payload xc.TxDataToSign) (xc.TxSignature, error) {
	privateKey := ed25519.NewKeyFromSeed(privateKeyBz)
	sig := ed25519.Sign(privateKey, payload)
	return sig, nil
}

func (signer Signer) PublicKey(privateKeyBz xc.PrivateKey) (xc.PublicKey, error) {
	privateKey := ed25519.NewKeyFromSeed(privateKeyBz)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return xc.PublicKey(publicKey), nil
}

type Sr25519Signer struct {
}

func signingContext(msg []byte) *merlin.Transcript {
	return sr25519.NewSigningContext([]byte("substrate"), msg)
}

func (signer Sr25519Signer) Sign(privateKey xc.PrivateKey, payload xc.TxDataToSign) (xc.TxSignature, error) {
	secret := [32]byte{}
	copy(secret[:], privateKey)
	ms, err := sr25519.NewMiniSecretKeyFromRaw(secret)
	if err != nil {
		return nil, err
	}
	key := ms.ExpandEd25519()
	sig, err := key.Sign(signingContext(payload))
	if err != nil {
		return nil, err
	}
	sigEncoded := sig.Encode()
	return sigEncoded[:], nil
}

func (signer Sr25519Signer) PublicKey(privateKeyBz xc.PrivateKey) (xc.PublicKey, error) {
	secret := [32]byte{}
	copy(secret[:], privateKeyBz)
	ms, err := sr25519.NewMiniSecretKeyFromRaw(secret)
	if err != nil {
		return nil, err
	}
	key := ms.ExpandEd25519()
	public, err := key.Public()
	if err != nil {
		return nil, err
	}
	publicEncoded := public.Encode()
	return publicEncoded[:], nil
}
