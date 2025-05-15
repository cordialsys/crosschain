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
	// _ = dagconfig.MainnetParams.Prefix
	// pubKeyAddress, err := util.NewAddressPublicKey(schnorr.SerializePubKey(pubKey), util.Bech32Prefix(ab.prefix))
	// if err != nil {
	// 	return "", err
	// }
	asStr := bech32.Encode(ab.prefix, schnorr.SerializePubKey(pubKey), 0)

	// Everyone has their own bech32 encoding algorithm
	// addr := pubKeyAddress.EncodeAddress()

	return xc.Address(asStr), nil
}
