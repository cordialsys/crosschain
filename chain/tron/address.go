package tron

import (
	"encoding/hex"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/okx/go-wallet-sdk/coins/tron"
)

// AddressBuilder for Template
type AddressBuilder struct {
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) < 32 {
		return "", fmt.Errorf("invalid secp256k1 public key")
	}
	address, err := tron.GetAddressByPublicKey(hex.EncodeToString(publicKeyBytes))
	return xc.Address(address), err
}
