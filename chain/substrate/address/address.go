package address

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/vedhavyas/go-subkey/v2"
)

// AddressBuilder for Template
type AddressBuilder struct {
	chainPrefix uint16
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	prefix := cfgI.ChainPrefix
	prefixNum, ok := prefix.AsInt()
	if !ok {
		return nil, fmt.Errorf("expecting numeric byte for chain_prefix for substrate chain '%s': %s", cfgI.Chain, prefix)
	}
	return AddressBuilder{chainPrefix: uint16(prefixNum)}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) == 33 {
		// drop address identifier?
		publicKeyBytes = publicKeyBytes[1:]
	}
	// Converts from ed25519 (32 bytes) to SS58
	if len(publicKeyBytes) != 32 {
		fmt.Println(publicKeyBytes[0], "vs", ab.chainPrefix)
		return xc.Address(""), fmt.Errorf("invalid ed25519 public key, expecting %d bytes but got %d", 32, len(publicKeyBytes))
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

func DecodeMulti(addr xc.Address) (types.MultiAddress, error) {
	decodedVal := base58.Decode(string(addr))
	if len(decodedVal) < 34 {
		return types.MultiAddress{}, fmt.Errorf("address %s is too short", addr)
	}
	newAddr, err := types.NewMultiAddressFromAccountID(last32DropChecksum(decodedVal))
	if err != nil {
		return types.MultiAddress{}, fmt.Errorf("invalid address %s: %v", addr, err)
	}
	return newAddr, nil
}

// Decoding address without checking the checksum
func last32DropChecksum(decoded []byte) []byte {
	// drop the 2 checksum bytes
	decoded = decoded[:len(decoded)-2]
	// take the last 32 bytes (ignores the 1-2 byte prefix)
	return decoded[len(decoded)-32:]
}

func Decode(addr xc.Address) (*types.AccountID, error) {

	decodedVal := base58.Decode(string(addr))
	if len(decodedVal) < 34 {
		return &types.AccountID{}, fmt.Errorf("address %s is too short", addr)
	}
	newAddr, err := types.NewAccountID(last32DropChecksum(decodedVal))
	if err != nil {
		return &types.AccountID{}, fmt.Errorf("invalid address %s: %v", addr, err)
	}
	return newAddr, nil
}
