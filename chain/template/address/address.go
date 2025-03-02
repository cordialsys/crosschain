package address

import (
	"errors"

	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Template
type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, errors.New("not implemented")
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	return xc.Address(""), errors.New("not implemented")
}
