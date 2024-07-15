package address

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

// TON prescibes using this subwallet for importing compatibility
const DefaultSubwalletId = 698983191

// Most stable TON wallet version
const DefaultWalletVersion = wallet.V3

// AddressBuilder for Template
type AddressBuilder struct {
	Asset xc.ITask
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI xc.ITask) (xc.AddressBuilder, error) {
	return AddressBuilder{cfgI}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	addr, err := wallet.AddressFromPubKey(publicKeyBytes, DefaultWalletVersion, DefaultSubwalletId)
	if err != nil {
		return "", err
	}
	if ab.Asset.GetChain().Net == "testnet" {
		addr.SetTestnetOnly(true)
	}
	return xc.Address(addr.String()), nil
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

func ParseAddress(addr xc.Address) (*address.Address, error) {
	addrS := string(addr)
	if len(strings.Split(addrS, ":")) == 2 {
		return address.ParseRawAddr(addrS)
	}

	return address.ParseAddr(addrS)
}

func Normalize(address string) (string, error) {
	addr, err := ParseAddress(xc.Address(address))
	if err != nil {
		return address, err
	}
	return addr.String(), nil
}
