package address

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cosmos/btcutil/bech32"
	"golang.org/x/crypto/blake2b"
)

const (
	AddressTypePaymentKeyHash      = 0b0110_0000
	AddressTypePaymentStakeKeyHash = 0b0000_0000
	AddressTypeStake               = 0b1110_0000
	// TODO: Deprecate legacy format
	AddressFormatLegacy  = "legacy"  // address that doesn't contain stake key hash
	AddressFormatPayment = "payment" // address that consists of payment + stake key hash
	AddressFormatStake   = "stake"   // stake address
	NetworkTagTestnet    = 0b0000_0000
	NetworkTagMainnet    = 0b0000_0001
)

// AddressBuilder for Template
type AddressBuilder struct {
	IsMainnet bool
	Format    xc.AddressFormat
}

var _ xc.AddressBuilder = AddressBuilder{}
var _ xc.AddressBuilderWithFormats = AddressBuilder{}

func InvalidAddressf(got xc.AddressFormat) error {
	return fmt.Errorf(
		"invalid address format, expected one of: [%s, %s, %s], got: %s",
		AddressFormatLegacy, AddressFormatPayment, AddressFormatStake, got,
	)
}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	opts, err := xcaddress.NewAddressOptions(options...)
	if err != nil {
		return AddressBuilder{}, err
	}

	format, ok := opts.GetFormat()
	if ok && format != AddressFormatLegacy && format != AddressFormatPayment && format != AddressFormatStake {
		return nil, InvalidAddressf(format)
	}

	if !ok {
		format = AddressFormatPayment
	}
	return AddressBuilder{
		IsMainnet: cfgI.Network == "" || strings.ToLower(cfgI.Network) == "mainnet",
		Format:    format,
	}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) != 32 {
		return "", fmt.Errorf("invalid public key length: %d, expected 32", len(publicKeyBytes))
	}

	if ab.Format == AddressFormatPayment || ab.Format == AddressFormatLegacy {
		withStakeKeyHash := ab.Format == AddressFormatPayment
		return GetPaymentAddress(publicKeyBytes, withStakeKeyHash, ab.IsMainnet)
	} else if ab.Format == AddressFormatStake {
		return GetStakeAddress(publicKeyBytes, ab.IsMainnet)
	}

	return xc.Address(""), InvalidAddressf(ab.Format)
}

func GetPaymentAddress(publicKeyBytes []byte, withStakeKeyHash bool, isMainnet bool) (xc.Address, error) {
	// Header consists of 2 parts:
	// - [0..4] bits: Address type
	// - [4..8] bits: Network tag
	var header byte
	var hrm string

	if withStakeKeyHash {
		header = AddressTypePaymentStakeKeyHash
	} else {
		header = AddressTypePaymentKeyHash
	}

	if isMainnet {
		hrm = "addr"
		header |= NetworkTagMainnet
	} else {
		hrm = "addr_test"
		header |= NetworkTagTestnet
	}

	b2b, err := blake2b.New(28, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create blake2b hash: %w", err)
	}
	b2b.Write(publicKeyBytes)
	addrHash := b2b.Sum(nil)

	addrBytes := []byte{header}
	addrBytes = append(addrBytes, addrHash...)
	if withStakeKeyHash {
		// use the same public key for stake delegation key hash
		addrBytes = append(addrBytes, addrHash...)
	}

	bech, err := bech32.EncodeFromBase256(hrm, addrBytes)
	if err != nil {
		return "", fmt.Errorf("failed to encode address: %w", err)
	}

	return xc.Address(bech), nil
}

func GetStakeAddress(publicKeyBytes []byte, isMainnet bool) (xc.Address, error) {
	var hrm string
	// Header consists of 2 parts:
	// - [0..4] bits: Address type
	// - [4..8] bits: Network tag
	var header byte

	if isMainnet {
		hrm = "stake"
		header = AddressTypeStake | NetworkTagMainnet
	} else {
		hrm = "stake_test"
		header = AddressTypeStake | NetworkTagTestnet
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

func (ab AddressBuilder) GetSignatureAlgorithm() xc.SignatureType {
	return xc.ADA.Driver().SignatureAlgorithms()[0]
}

func GetKeyHash(bytes []byte) ([]byte, error) {
	b2b, err := blake2b.New(28, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blake2b hash: %w", err)
	}
	b2b.Write(bytes)
	return b2b.Sum(nil), nil
}
