package substrate

import (
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/crypto/blake2b"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Template
type AddressBuilder struct {
	chainPrefix byte
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI xc.ITask) (xc.AddressBuilder, error) {
	prefix := cfgI.GetChain().ChainPrefix
	prefixNum, err := strconv.ParseUint(prefix, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("expecting numeric byte for chain_prefix for substrate chain %s: %v", cfgI.GetChain().Chain, err)
	}
	return AddressBuilder{chainPrefix: byte(prefixNum)}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	// Converts from sr25519 (32 bytes) to SS58
	if len(publicKeyBytes) != 32 {
		return xc.Address(""), errors.New("invalid sr25519 public key")
	}

	publicKeyBytes = append([]byte{ab.chainPrefix}, publicKeyBytes...)
	preimage := append([]byte("SS58PRE"), publicKeyBytes...)
	checksum := blake2b.Sum512(preimage)
	publicKeyBytes = append(publicKeyBytes, checksum[:2]...)
	return xc.Address(base58.Encode(publicKeyBytes)), nil
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
