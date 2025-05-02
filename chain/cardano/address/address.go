package address

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/btcutil/bech32"
	"golang.org/x/crypto/blake2b"
)

const (
	AddressTypePaymentKeyHash = 0b0110_0000
	NetworkTagTestnet         = 0b0000_0000
	NetworkTagMainnet         = 0b0000_0001
)

// AddressBuilder for Template
type AddressBuilder struct {
	IsMainnet bool
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{
		IsMainnet: cfgI.Network == "" || strings.ToLower(cfgI.Network) == "mainnet",
	}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	var hrm string
	// Header consists of 2 parts:
	// - [0..4] bits: Address type
	// - [4..8] bits: Network tag
	var header byte

	if ab.IsMainnet {
		hrm = "addr"
		header = AddressTypePaymentKeyHash | NetworkTagMainnet
	} else {
		hrm = "addr_test"
		header = AddressTypePaymentKeyHash | NetworkTagTestnet
	}

	b2b, err := blake2b.New(28, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create blake2b hash: %w", err)
	}
	b2b.Write(publicKeyBytes)
	addrHash := b2b.Sum(nil)

	addrBytes := []byte{header}
	addrBytes = append(addrBytes, addrHash...)

	bech, err := bech32.EncodeFromBase256(hrm, addrBytes)
	if err != nil {
		return "", fmt.Errorf("failed to encode address: %w", err)
	}

	return xc.Address(bech), nil
}
