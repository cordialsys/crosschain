package address

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/btcutil/bech32"
)

const (
	HRP = "erd"
)

type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) != 32 {
		return xc.Address(""), fmt.Errorf("expected public key length 32, got %d", len(publicKeyBytes))
	}

	bech32Addr, err := bech32.EncodeFromBase256(HRP, publicKeyBytes)
	return xc.Address(bech32Addr), err
}
