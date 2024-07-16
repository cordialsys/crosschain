package substrate

import (
	"errors"
	"fmt"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	"github.com/vedhavyas/go-subkey/v2"
)

// AddressBuilder for Template
type AddressBuilder struct {
	chainPrefix uint16
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI xc.ITask) (xc.AddressBuilder, error) {
	prefix := cfgI.GetChain().ChainPrefix
	prefixNum, err := strconv.ParseUint(prefix, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("expecting numeric byte for chain_prefix for substrate chain %s: %v", cfgI.GetChain().Chain, err)
	}
	return AddressBuilder{chainPrefix: uint16(prefixNum)}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	// Converts from sr25519 (32 bytes) to SS58
	if len(publicKeyBytes) != 32 {
		return xc.Address(""), errors.New("invalid sr25519 public key")
	}
	addr := subkey.SS58Encode(publicKeyBytes, ab.chainPrefix)
	return xc.Address(addr), nil
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
