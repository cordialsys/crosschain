package address

import (
	"errors"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	log "github.com/sirupsen/logrus"
)

type AddressType string

const (
	AddressTypeLegacy  = "legacy"
	AddressTypeSegWit  = "segwit"
	AddressTypeTaproot = "taproot"
)

// AddressBuilder for Bitcoin
type AddressBuilder struct {
	params    *chaincfg.Params
	asset     xc.ITask
	algorithm xc.SignatureType
}

var _ xc.AddressBuilder = &AddressBuilder{}

type AddressDecoder interface {
	Decode(to xc.Address, params *chaincfg.Params) (btcutil.Address, error)
}

type WithAddressDecoder interface {
	WithAddressDecoder(decoder AddressDecoder) WithAddressDecoder
}

type BtcAddressDecoder struct{}

var _ AddressDecoder = &BtcAddressDecoder{}

func (*BtcAddressDecoder) Decode(addr xc.Address, params *chaincfg.Params) (btcutil.Address, error) {
	return btcutil.DecodeAddress(string(addr), params)
}

func NewAddressDecoder() *BtcAddressDecoder {
	return &BtcAddressDecoder{}
}

// NewAddressBuilder creates a new Bitcoin AddressBuilder
func NewAddressBuilder(asset xc.ITask, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	opts, err := xcaddress.NewAddressOptions(options...)
	if err != nil {
		return AddressBuilder{}, err
	}

	params, err := params.GetParams(asset.GetChain())
	if err != nil {
		return AddressBuilder{}, err
	}

	algorithm, ok := opts.GetAlgorithmType()
	// Check if default algorithm is overridden
	if ok {
		if algorithm != xc.Schnorr && algorithm != xc.K256Sha256 {
			return AddressBuilder{}, fmt.Errorf("unsupported address type: %s", algorithm)
		}
	} else {
		algorithm = asset.GetChain().Driver.SignatureAlgorithm()
	}

	log.Debugf("New bitcoin address builder with algorithm: %s", algorithm)
	return AddressBuilder{
		asset:     asset,
		params:    params,
		algorithm: algorithm,
	}, nil
}

func (ab AddressBuilder) GetAddressType() (AddressType, error) {
	if ab.algorithm == xc.Schnorr {
		return AddressTypeTaproot, nil
	}

	if ab.algorithm == "" || ab.algorithm == xc.K256Sha256 {
		if ab.asset.GetChain().Driver == xc.DriverBitcoinLegacy {
			return AddressTypeLegacy, nil
		} else {
			return AddressTypeSegWit, nil
		}
	} else {
		return AddressType(""), fmt.Errorf("bitcoin doesn't support %s address type", ab.algorithm)
	}
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

func (ab AddressBuilder) GetTaprootAddress(publicKey []byte) (xc.Address, error) {
	keyLen := len(publicKey)
	var normalizedKey []byte
	if keyLen == 32 {
		normalizedKey = publicKey[:]
	} else if keyLen == 33 {
		normalizedKey = publicKey[1:]
	} else {
		return "", fmt.Errorf("invalid key length, taproot only supports compressed public keys (32/33 bytes), got: %d", keyLen)
	}

	taprootAddress, err := btcutil.NewAddressTaproot(normalizedKey, ab.params)
	if err != nil {
		return "", err
	}
	address := taprootAddress.EncodeAddress()
	return xc.Address(address), nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	addressType, err := ab.GetAddressType()
	if err != nil {
		return "", err
	}

	pubkey, err := btcec.ParsePubKey(publicKeyBytes)
	if err != nil {
		return "", err
	}

	// force compressed format, BTC wallets should use uncompressed.
	publicKeyBytes = pubkey.SerializeCompressed()
	switch addressType {
	case AddressTypeLegacy:
		return ab.GetLegacyAddress(publicKeyBytes)
	case AddressTypeSegWit:
		return ab.GetSegWitAddress(publicKeyBytes)
	case AddressTypeTaproot:
		return ab.GetTaprootAddress(publicKeyBytes)
	default:
		return "", errors.New("failed to determine bitcoin address type")
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
