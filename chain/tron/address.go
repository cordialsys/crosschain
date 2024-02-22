package tron

import (
	"crypto/ecdsa"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
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
	var publicKey *ecdsa.PublicKey
	var err error
	publicKey, err = crypto.UnmarshalPubkey(publicKeyBytes)
	if err != nil {
		return xc.Address(""), err
	}

	address := address.PubkeyToAddress(*publicKey).String()
	return xc.Address(address), nil
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
