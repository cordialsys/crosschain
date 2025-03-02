package bitcoin_cash

import (
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
	xc "github.com/cordialsys/crosschain"
	bitcoinaddress "github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cosmos/btcutil/bech32"
	"golang.org/x/crypto/ripemd160"
)

// AddressBuilder for Bitcoin
type AddressBuilder struct {
	params *chaincfg.Params
	asset  *xc.ChainBaseConfig
}

var _ xc.AddressBuilder = &AddressBuilder{}

type BchAddressDecoder struct{}

func NewAddressDecoder() *BchAddressDecoder {
	return &BchAddressDecoder{}
}

var _ bitcoinaddress.AddressDecoder = &BchAddressDecoder{}

func (*BchAddressDecoder) Decode(inputAddr xc.Address, params *chaincfg.Params) (btcutil.Address, error) {
	addr, err := btcutil.DecodeAddress(string(inputAddr), params)
	if err != nil {
		// try to decode as BCH
		bchaddr, err2 := DecodeBchAddress(string(inputAddr), params)
		if err2 != nil {
			return nil, errors.Join(err, err2)
		}
		addr, err2 = BchAddressFromBytes(bchaddr, params)
		if err2 != nil {
			return nil, errors.Join(err, err2)
		}
	}
	return addr, nil
}

var (
	// Alphabet used by Bitcoin Cash to encode addresses.
	Alphabet = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	// AlphabetReverseLookup used by Bitcoin Cash to decode addresses.
	AlphabetReverseLookup = func() map[rune]byte {
		lookup := map[rune]byte{}
		for i, char := range Alphabet {
			lookup[char] = byte(i)
		}
		return lookup
	}()
)

// NewAddressBuilder creates a new Bitcoin AddressBuilder
func NewAddressBuilder(asset *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	params, err := params.GetParams(asset)
	if err != nil {
		return AddressBuilder{}, err
	}
	return AddressBuilder{
		asset:  asset,
		params: params,
	}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	addressPubKey, err := NewBchAddressPubKey(publicKeyBytes, ab.params)
	if err != nil {
		return "", err
	}
	prefix := AddressPrefix(ab.params)
	hash := *addressPubKey.Hash160()
	encoded, err := encodeBchAddress(0x00, hash[:], ab.params)
	if err != nil {
		return "", err
	}
	// legacy format
	// encoded = addressPubKey.EncodeAddress()
	return xc.Address(prefix + ":" + encoded), nil
}

func (ab AddressBuilder) GetLegacyAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	addressPubKey, err := NewBchAddressPubKey(publicKeyBytes, ab.params)
	if err != nil {
		return "", err
	}
	prefix := AddressPrefix(ab.params)
	// legacy format
	encoded := addressPubKey.EncodeAddress()
	return xc.Address(prefix + ":" + encoded), nil
}

// GetAllPossibleAddressesFromPublicKey returns all PossubleAddress(es) given a public key
func (ab AddressBuilder) GetAllPossibleAddressesFromPublicKey(publicKeyBytes []byte) ([]xc.PossibleAddress, error) {

	addr, err := ab.GetAddressFromPublicKey(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	legacy, err := ab.GetLegacyAddressFromPublicKey(publicKeyBytes)
	if err != nil {
		return nil, err
	}

	return []xc.PossibleAddress{
		{
			Address: addr,
		},
		{
			Address: legacy,
		},
	}, nil
}

func BchAddressFromBytes(addrBytes []byte, params *chaincfg.Params) (btcutil.Address, error) {
	switch len(addrBytes) - 1 {
	case ripemd160.Size: // P2PKH or P2SH
		switch addrBytes[0] {
		case 0: // P2PKH
			return btcutil.NewAddressPubKeyHash(addrBytes[1:21], params)
		case 8: // P2SH
			return btcutil.NewAddressScriptHashFromHash(addrBytes[1:21], params)
		default:
			return nil, btcutil.ErrUnknownAddressType
		}
	default:
		addr, err := btcutil.DecodeAddress(base58.Encode(addrBytes), params)
		if err != nil {
			return nil, err
		}
		return addr, nil
	}
}

func decodeLegacyAddress(addr string, params *chaincfg.Params) ([]byte, error) {
	// Decode the checksummed base58 format address.
	decoded, ver, err := base58.CheckDecode(addr)
	if err != nil {
		return nil, fmt.Errorf("checking: %v", err)
	}
	if len(decoded) != 20 {
		return nil, fmt.Errorf("expected len 20, got len %v", len(decoded))
	}

	// Validate the address format.
	switch ver {
	case params.PubKeyHashAddrID, params.ScriptHashAddrID:
		return base58.Decode(string(addr)), nil
	default:
		return nil, fmt.Errorf("unexpected address prefix")
	}
}

// DecodeAddress implements the address.Decoder interface
func DecodeBchAddress(addr string, params *chaincfg.Params) ([]byte, error) {
	// Legacy address decoding
	if legacyAddr, err := btcutil.DecodeAddress(addr, params); err == nil {
		switch legacyAddr.(type) {
		case *btcutil.AddressPubKeyHash, *btcutil.AddressScriptHash, *btcutil.AddressPubKey:
			return decodeLegacyAddress(addr, params)
		case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressWitnessScriptHash:
			return nil, fmt.Errorf("unsuported segwit bitcoin address type %T", legacyAddr)
		default:
			return nil, fmt.Errorf("unsuported legacy bitcoin address type %T", legacyAddr)
		}
	}

	if addrParts := strings.Split(string(addr), ":"); len(addrParts) != 1 {
		addr = addrParts[1]
	}

	decoded := DecodeBchString(string(addr))
	if !VerifyChecksum(AddressPrefix(params), decoded) {
		return nil, btcutil.ErrChecksumMismatch
	}

	addrBytes, err := bech32.ConvertBits(decoded[:len(decoded)-8], 5, 8, false)
	if err != nil {
		return nil, err
	}

	switch len(addrBytes) - 1 {
	case ripemd160.Size: // P2PKH or P2SH
		switch addrBytes[0] {
		case 0, 8: // P2PKH or P2SH
			return addrBytes, nil
		default:
			return nil, btcutil.ErrUnknownAddressType
		}
	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

// DecodeString using Bitcoin Cash address encoding.
func DecodeBchString(address string) []byte {
	data := []byte{}
	for _, c := range address {
		data = append(data, AlphabetReverseLookup[c])
	}
	return data
}

// Verify if a bch payload is well formed
func VerifyChecksum(prefix string, payload []byte) bool {
	return PolyMod(append(EncodePrefix(prefix), payload...)) == 0
}

// PolyMod is the checksum alg for BCH
// https://github.com/bitcoincashorg/bitcoincash.org/blob/master/spec/cashaddr.md
func PolyMod(v []byte) uint64 {
	c := uint64(1)
	for _, d := range v {
		c0 := byte(c >> 35)
		c = ((c & 0x07ffffffff) << 5) ^ uint64(d)

		if c0&0x01 > 0 {
			c ^= 0x98f2bc8e61
		}
		if c0&0x02 > 0 {
			c ^= 0x79b76d99e2
		}
		if c0&0x04 > 0 {
			c ^= 0xf33e5fb3c4
		}
		if c0&0x08 > 0 {
			c ^= 0xae2eabe2a8
		}
		if c0&0x10 > 0 {
			c ^= 0x1e4f43e470
		}
	}
	return c ^ 1
}

// The bch prefix is different for each network type
func AddressPrefix(params *chaincfg.Params) string {
	if params == nil {
		panic(fmt.Errorf("non-exhaustive pattern: params %v", params))
	}
	switch params.Name {
	case chaincfg.MainNetParams.Name:
		return "bitcoincash"
	case chaincfg.TestNet3Params.Name:
		return "bchtest"
	case chaincfg.RegressionNetParams.Name:
		return "bchreg"
	default:
		panic(fmt.Errorf("non-exhaustive pattern: params %v", params.Name))
	}
}

// https://github.com/bitcoincashorg/bitcoincash.org/blob/master/spec/cashaddr.md#checksum
func EncodePrefix(prefixString string) []byte {
	prefixBytes := make([]byte, len(prefixString)+1)
	for i := 0; i < len(prefixString); i++ {
		prefixBytes[i] = byte(prefixString[i]) & 0x1f
	}
	prefixBytes[len(prefixString)] = 0
	return prefixBytes
}

// Create a new bch, btc compatible address pkh
func NewBchAddressPubKey(pk []byte, params *chaincfg.Params) (*btcutil.AddressPubKeyHash, error) {
	addr, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), params)
	return addr, err
}

// encodeAddress using Bitcoin Cash address encoding, assuming that the hash
// data has no prefix or checksum.
func encodeBchAddress(version byte, hash []byte, params *chaincfg.Params) (string, error) {
	if (len(hash)-20)/4 != int(version)%8 {
		return "", fmt.Errorf("invalid version: %d", version)
	}
	data, err := bech32.ConvertBits(append([]byte{version}, hash...), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("invalid bech32 encoding: %v", err)
	}
	return EncodeToBchString(AppendBchChecksum(AddressPrefix(params), data)), nil
}

// White bch data as a bch encoded string
func EncodeToBchString(data []byte) string {
	addr := strings.Builder{}
	for _, d := range data {
		addr.WriteByte(Alphabet[d])
	}
	return addr.String()
}

// Add bch checksum to a payload
// https://github.com/bitcoincashorg/bitcoincash.org/blob/master/spec/cashaddr.md#checksum
func AppendBchChecksum(prefix string, payload []byte) []byte {
	prefixedPayload := append(EncodePrefix(prefix), payload...)

	// Append 8 zeroes.
	prefixedPayload = append(prefixedPayload, 0, 0, 0, 0, 0, 0, 0, 0)

	// Determine what to XOR into those 8 zeroes.
	mod := PolyMod(prefixedPayload)

	checksum := make([]byte, 8)
	for i := 0; i < 8; i++ {
		// Convert the 5-bit groups in mod to checksum values.
		checksum[i] = byte((mod >> uint(5*(7-i))) & 0x1f)
	}
	return append(payload, checksum...)
}
