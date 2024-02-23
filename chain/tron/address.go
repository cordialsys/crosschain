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
func NewAddressBuilder(cfgI xc.ITask) (xc.AddressBuilder, error) {
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

// GetAllPossibleAddressesFromPublicKey returns all PossubleAddress(es) given a public key
func (ab AddressBuilder) GetAllPossibleAddressesFromPublicKey(publicKeyBytes []byte) ([]xc.PossibleAddress, error) {
	address, err := ab.GetAddressFromPublicKey(publicKeyBytes)
	return []xc.PossibleAddress{
		{
			Address: address,
			Type:    xc.AddressTypeDefault,
		},
	}, err
}
