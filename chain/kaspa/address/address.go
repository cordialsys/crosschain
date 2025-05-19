package address

import (
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	xc "github.com/cordialsys/crosschain"
	"github.com/kaspanet/kaspad/util/bech32"
)

type AddressBuilder struct {
	prefix string
}

var _ xc.AddressBuilder = AddressBuilder{}

func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	prefix := cfgI.ChainPrefix.AsString()
	return AddressBuilder{
		prefix,
	}, nil
}

func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	pubKey, err := schnorr.ParsePubKey(publicKeyBytes)
	if err != nil {
		return "", err
	}
	// Everyone has their own bech32 encoding algorithm
	asStr := bech32.Encode(ab.prefix, schnorr.SerializePubKey(pubKey), 0)

	return xc.Address(asStr), nil
}
