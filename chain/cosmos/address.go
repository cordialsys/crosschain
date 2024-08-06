package cosmos

import (
	xc "github.com/cordialsys/crosschain"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AddressBuilder for Cosmos
type AddressBuilder struct {
	Asset xc.ITask
}

// NewAddressBuilder creates a new Cosmos AddressBuilder
func NewAddressBuilder(asset xc.ITask) (xc.AddressBuilder, error) {
	return AddressBuilder{
		Asset: asset,
	}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	publicKey := getPublicKey(ab.Asset.GetChain(), publicKeyBytes)
	rawAddress := publicKey.Address()

	err := sdk.VerifyAddressFormat(rawAddress)
	if err != nil {
		return xc.Address(""), err
	}
	bech32Addr, err := sdk.Bech32ifyAddressBytes(ab.Asset.GetChain().ChainPrefix, rawAddress)
	return xc.Address(bech32Addr), err
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