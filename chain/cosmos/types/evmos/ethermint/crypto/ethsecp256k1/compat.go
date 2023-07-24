package ethsecp256k1

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"

	tmcrypto "github.com/cometbft/cometbft/crypto"
	"github.com/cosmos/cosmos-sdk/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	PrivKeySizeBytes = 32
	PubKeySizeBytes  = 33
	KeyAlgo          = "eth_secp256k1"
)

var (
	_ cryptotypes.PrivKey  = &PrivKey{}
	_ codec.AminoMarshaler = &PrivKey{}
	_ cryptotypes.PubKey   = &PubKey{}
	_ codec.AminoMarshaler = &PubKey{}
)

func GenerateKey() (*PrivKey, error) {
	return nil, errors.New("not supported in crosschain")
}

func (privKey PrivKey) Bytes() []byte {
	bz := make([]byte, len(privKey.Key))
	copy(bz, privKey.Key)

	return bz
}

func (privKey PrivKey) PubKey() cryptotypes.PubKey {
	pk, err := privKey.ToECDSA()
	if err != nil {
		return nil
	}

	return &PubKey{
		Key: crypto.CompressPubkey(&pk.PublicKey),
	}
}

func (privKey PrivKey) Equals(other cryptotypes.LedgerPrivKey) bool {
	return bytes.Equal(privKey.Bytes(), other.Bytes())
}

func (privKey PrivKey) Type() string {
	return KeyAlgo
}

func (privKey PrivKey) MarshalAmino() ([]byte, error) {
	return privKey.Key, nil
}

func (privKey *PrivKey) UnmarshalAmino(bz []byte) error {
	privKey.Key = bz

	return nil
}

func (privKey PrivKey) MarshalAminoJSON() ([]byte, error) {
	return privKey.MarshalAmino()
}

func (privKey *PrivKey) UnmarshalAminoJSON(bz []byte) error {
	return privKey.UnmarshalAmino(bz)
}

func (privKey PrivKey) Sign(digestBz []byte) ([]byte, error) {
	return nil, errors.New("not supported in crosschain")
}

func (privKey PrivKey) ToECDSA() (*ecdsa.PrivateKey, error) {
	return crypto.ToECDSA(privKey.Bytes())
}

func (pubKey PubKey) Address() tmcrypto.Address {
	pubk, err := crypto.DecompressPubkey(pubKey.Key)
	if err != nil {
		return nil
	}

	return tmcrypto.Address(crypto.PubkeyToAddress(*pubk).Bytes())
}

func (pubKey PubKey) Bytes() []byte {
	bz := make([]byte, len(pubKey.Key))
	copy(bz, pubKey.Key)

	return bz
}

func (pubKey PubKey) String() string {
	return fmt.Sprintf("EthPubKeySecp256k1{%X}", pubKey.Key)
}

func (pubKey PubKey) Type() string {
	return KeyAlgo
}

func (pubKey PubKey) Equals(other cryptotypes.PubKey) bool {
	return bytes.Equal(pubKey.Bytes(), other.Bytes())
}

func (pubKey PubKey) MarshalAmino() ([]byte, error) {
	return pubKey.Key, nil
}

func (pubKey *PubKey) UnmarshalAmino(bz []byte) error {
	pubKey.Key = bz

	return nil
}

func (pubKey PubKey) MarshalAminoJSON() ([]byte, error) {
	return pubKey.MarshalAmino()
}

func (pubKey *PubKey) UnmarshalAminoJSON(bz []byte) error {
	return pubKey.UnmarshalAmino(bz)
}

func (pubKey PubKey) VerifySignature(msg, sig []byte) bool {
	if len(sig) == crypto.SignatureLength {
		// remove recovery byte
		sig = sig[:len(sig)-1]
	}

	return crypto.VerifySignature(pubKey.Key, crypto.Keccak256Hash(msg).Bytes(), sig)
}
