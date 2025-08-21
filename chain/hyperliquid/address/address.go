package address

import (
	"crypto/ecdsa"
	"errors"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/crypto"
)

// AddressBuilder for EVM
type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	var publicKey *ecdsa.PublicKey
	var err error
	if len(publicKeyBytes) == 33 {
		publicKey, err = crypto.DecompressPubkey(publicKeyBytes)
		if err != nil {
			return xc.Address(""), errors.New("invalid k256 public key")
		}
	} else {
		publicKey, err = crypto.UnmarshalPubkey(publicKeyBytes)
		if err != nil {
			return xc.Address(""), err
		}
	}

	address := crypto.PubkeyToAddress(*publicKey).Hex()
	// Lowercase the address is our normalized format
	return xc.Address(strings.ToLower(address)), nil
}
