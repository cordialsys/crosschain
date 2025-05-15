package address

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AddressBuilder for Cosmos
type AddressBuilder struct {
	Asset *xc.ChainBaseConfig
}

// NewAddressBuilder creates a new Cosmos AddressBuilder
func NewAddressBuilder(asset *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{
		Asset: asset,
	}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	publicKey := GetPublicKey(ab.Asset, publicKeyBytes)
	rawAddress := publicKey.Address()
	if len(rawAddress) == 0 {
		return "", fmt.Errorf("address cannot be empty")
	}

	bech32Addr, err := sdk.Bech32ifyAddressBytes(string(ab.Asset.ChainPrefix), rawAddress)
	return xc.Address(bech32Addr), err
}
