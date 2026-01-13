package address

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Near
type AddressBuilder struct{}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Near AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	hexPk := hex.EncodeToString(publicKeyBytes)
	return xc.Address(hexPk), nil
}
