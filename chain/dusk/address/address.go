package address

import (
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/cloudflare/circl/sign/bls"
	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Template
type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	return xc.Address(base58.Encode(publicKeyBytes)), nil
}

func GetPublicKeyFromAddress(address xc.Address) (bls.PublicKey[bls.G2], error) {
	bytes := base58.Decode(string(address))

	var publicKey bls.PublicKey[bls.G2]
	err := publicKey.UnmarshalBinary(bytes)

	return publicKey, err
}
