package tron

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"golang.org/x/crypto/sha3"
)

// AddressBuilder for Template
type AddressBuilder struct {
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

func GetAddressByPublicKey(pubKey string) (string, error) {
	pubKeyByte, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key")
	}

	pk, err := btcec.ParsePubKey(pubKeyByte)
	uncompressedPubKey := pk.SerializeUncompressed()
	if err != nil {
		return "", fmt.Errorf("pubKey encoding err ")
	}

	h := sha3.NewLegacyKeccak256()
	h.Write(uncompressedPubKey[1:])
	hash := h.Sum(nil)[12:]
	network := byte(0x41)
	return base58.CheckEncode(hash, network), nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) < 32 {
		return "", fmt.Errorf("invalid secp256k1 public key")
	}
	address, err := GetAddressByPublicKey(hex.EncodeToString(publicKeyBytes))
	return xc.Address(address), err
}
