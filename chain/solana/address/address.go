package address

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Solana
type AddressBuilder struct {
}

// NewAddressBuilder creates a new Solana AddressBuilder
func NewAddressBuilder(asset *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) != 32 {
		return xc.Address(""), fmt.Errorf("expected address length 32, got address length %v", len(publicKeyBytes))
	}
	return xc.Address(base58.Encode(publicKeyBytes)), nil
}
