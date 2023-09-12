package substrate

import (
	"errors"

	"golang.org/x/crypto/blake2b"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Template
type AddressBuilder struct {
	chainID byte
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI xc.ITask) (xc.AddressBuilder, error) {
	return AddressBuilder{byte(cfgI.GetAssetConfig().ChainID)}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	// Converts from sr25519 (32 bytes) to SS58
	if len(publicKeyBytes) != 32 {
		return xc.Address(""), errors.New("invalid sr25519 public key")
	}

	publicKeyBytes = append([]byte{ab.chainID}, publicKeyBytes...)
	preimage := append([]byte{0x53, 0x53, 0x35, 0x38, 0x50, 0x52, 0x45}, publicKeyBytes...)
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
