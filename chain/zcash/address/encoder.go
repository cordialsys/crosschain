package address

import (
	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
)

type AddressBuilder struct {
	params *chaincfg.Params
	asset  *xc.ChainBaseConfig
}

var _ xc.AddressBuilder = &AddressBuilder{}

func NewAddressBuilder(asset *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	params, err := params.GetParams(asset)
	if err != nil {
		return AddressBuilder{}, err
	}

	// log.Debugf("New zcash address builder")
	builder := AddressBuilder{
		params,
		asset,
	}

	return &builder, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) == 32 {
		// btcec.ParsePubKey requires there be a compressed '2' header
		withCompressedHeader := make([]byte, 33)
		withCompressedHeader[0] = 0x02
		copy(withCompressedHeader[1:], publicKeyBytes)
		publicKeyBytes = withCompressedHeader
	}
	pubkey, err := btcec.ParsePubKey(publicKeyBytes)
	if err != nil {
		return "", err
	}

	// force compressed format, BTC wallets should use uncompressed.
	publicKeyBytes = pubkey.SerializeCompressed()
	hashBytes := [20]byte{}
	copy(hashBytes[:], btcutil.Hash160(publicKeyBytes))
	address := TAddress{
		hash:         hashBytes,
		netID:        ab.params.PubKeyHashAddrID,
		scriptHashId: ab.params.ScriptHashAddrID,
	}

	return xc.Address(address.EncodeAddress()), nil
}
