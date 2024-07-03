package bitcoin

import (
	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
)

// AddressBuilder for Bitcoin
type AddressBuilder struct {
	params *chaincfg.Params
	asset  xc.ITask
}

var _ xc.AddressBuilder = &AddressBuilder{}

// NewAddressBuilder creates a new Bitcoin AddressBuilder
func NewAddressBuilder(asset xc.ITask) (xc.AddressBuilder, error) {
	params, err := params.GetParams(asset.GetChain())
	if err != nil {
		return AddressBuilder{}, err
	}
	return AddressBuilder{
		asset:  asset,
		params: params,
	}, nil
}

func (ab AddressBuilder) GetLegacyAddress(publicKey []byte) (xc.Address, error) {
	address, err := btcutil.NewAddressPubKey(publicKey, ab.params)
	if err != nil {
		return "", err
	}

	return xc.Address(address.EncodeAddress()), nil
}
func (ab AddressBuilder) GetSegWitMultisigAddress(publicKey []byte) (xc.Address, error) {
	scriptHash := btcutil.Hash160(publicKey)
	addressPubKey, err := btcutil.NewAddressScriptHashFromHash(scriptHash, ab.params)
	if err != nil {
		return "", err
	}
	address := addressPubKey.EncodeAddress()
	return xc.Address(address), nil
}
func (ab AddressBuilder) GetSegWitAddress(publicKey []byte) (xc.Address, error) {
	scriptHash := btcutil.Hash160(publicKey)
	addressPubKey, err := btcutil.NewAddressWitnessPubKeyHash(scriptHash, ab.params)
	if err != nil {
		return "", err
	}
	address := addressPubKey.EncodeAddress()
	return xc.Address(address), nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	// // hack to support Taproot until btcutil is bumped
	// if len(publicKeyBytes) == 32 {
	// 	publicKeyBytes = append([]byte{0x02}, publicKeyBytes...)
	// }
	pubkey, err := btcec.ParsePubKey(publicKeyBytes)
	if err != nil {
		return "", err
	}
	// force compressed format, BTC wallets should use uncompressed.
	publicKeyBytes = pubkey.SerializeCompressed()
	if ab.asset.GetChain().Driver == xc.DriverBitcoinLegacy {
		return ab.GetLegacyAddress(publicKeyBytes)
	} else {
		return ab.GetSegWitAddress(publicKeyBytes)
	}
}

// GetAllPossibleAddressesFromPublicKey returns all PossubleAddress(es) given a public key
func (ab AddressBuilder) GetAllPossibleAddressesFromPublicKey(publicKeyBytes []byte) ([]xc.PossibleAddress, error) {

	possibles := []xc.PossibleAddress{}
	legacyAddress, err := ab.GetLegacyAddress(publicKeyBytes)
	if err != nil {
		return possibles, err
	}

	segwitAddress, err := ab.GetSegWitAddress(publicKeyBytes)
	if err != nil {
		return possibles, err
	}

	multiSigAddress, err := ab.GetSegWitMultisigAddress(publicKeyBytes)
	if err != nil {
		return possibles, err
	}

	return []xc.PossibleAddress{
		{
			Address: legacyAddress,
			Type:    xc.AddressTypeP2PKH,
		},
		{
			Address: segwitAddress,
			Type:    xc.AddressTypeP2WPKH,
		},
		{
			Address: multiSigAddress,
			Type:    "",
		},
	}, nil
}
