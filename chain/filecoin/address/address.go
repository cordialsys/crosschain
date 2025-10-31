package address

import (
	"bytes"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"

	"github.com/btcsuite/btcd/btcec/v2"
	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"golang.org/x/crypto/blake2b"
)

const (
	// An ID address
	ProtocolId = 0
	// A wallet address generated from a secp256k public key
	ProtocolSecp256k1 = 1
	// An actor address.
	ProtocolActor = 2
	// A wallet address generated from BLS public key.
	ProtocolBls = 3
	// A delegated address for user-defined foreign actors:
	// For example f410 is and Ethereum compatibile address space
	ProtocolDelegated    = 4
	EncodeStd            = "abcdefghijklmnopqrstuvwxyz234567"
	BlsPublicKeyBytesLen = 48
	// MaxSubaddressLen is the maximum length of a delegated address's sub-address.
	MaxSubaddressLen = 54
	// ChecksumHashLength defines the hash length used for calculating address checksums.
	ChecksumHashLength = 4

	payloadHashSize        = 20
	checksumHashSize       = 4
	protocolIdRawMaxLength = 20
)

var Encoding = base32.NewEncoding(EncodeStd)

// Filecoin address builder
type AddressBuilder struct {
	network   string
	alghoritm xc.SignatureType
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new address builder and validate provided algorithm.
// Default algorithm is specified in `Driver.SignatureAlgorithm()`
func NewAddressBuilder(asset *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	opts, err := xcaddress.NewAddressOptions(options...)
	if err != nil {
		return AddressBuilder{}, err
	}

	algorithm := asset.Driver.SignatureAlgorithms()[0]
	if alg, ok := opts.GetAlgorithmType(); ok {
		algorithm = alg
	}
	if algorithm != xc.K256Sha256 {
		return AddressBuilder{}, fmt.Errorf("unsupported address type: %s", algorithm)
	}

	return AddressBuilder{
		network:   asset.Network,
		alghoritm: algorithm,
	}, nil
}

func (ab AddressBuilder) GetSecp256k1Address(publicKeyBytes []byte) (xc.Address, error) {
	pk, err := btcec.ParsePubKey(publicKeyBytes)
	if err != nil {
		return xc.Address(""), fmt.Errorf("failed to parse public key: %w", err)
	}
	uncompressedPK := pk.SerializeUncompressed()
	pkHash, err := hash(uncompressedPK, payloadHashSize)
	if err != nil {
		return xc.Address(""), fmt.Errorf("failed pubkey hash: %w", err)
	}

	expectedLen := 1 + len(pkHash)
	buf := make([]byte, expectedLen)
	var addressType byte = ProtocolSecp256k1
	buf[0] = addressType
	copy(buf[1:], pkHash)

	checksum, err := hash(buf, checksumHashSize)
	if err != nil {
		return xc.Address(""), fmt.Errorf("failed checksum hash: %w", err)
	}

	prefix, err := ab.getPrefix()
	if err != nil {
		return xc.Address(""), err
	}
	address := prefix + fmt.Sprintf("%d", addressType) + Encoding.WithPadding(-1).EncodeToString(append(pkHash, checksum[:]...))

	return xc.Address(address), nil

}

// TODO: Implement once BLS keys are supported
func (ab AddressBuilder) GetBlsAddress(publicKeyBytes []byte) (xc.Address, error) {
	keyLen := len(publicKeyBytes)
	if keyLen != BlsPublicKeyBytesLen {
		return xc.Address(""), fmt.Errorf("invalid bls public key length: %v, expected: %v", keyLen, BlsPublicKeyBytesLen)
	}

	prefix, err := ab.getPrefix()
	if err != nil {
		return xc.Address(""), err
	}
	expectedLen := 1 + keyLen
	buf := make([]byte, expectedLen)
	var addressType byte = ProtocolBls
	buf[0] = addressType
	copy(buf[1:], publicKeyBytes)

	address := prefix + fmt.Sprintf("%d", addressType) + string(buf)
	return xc.Address(address), errors.New("not implementer")
}

func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	switch ab.alghoritm {
	case "bls":
		return xc.Address(""), errors.New("bls is currently unsupported")
		// TODO: Add support for BLS key generation
		// return ab.GetBlsAddress(publicKeyBytes)
	case "k256-sha256":
		return ab.GetSecp256k1Address(publicKeyBytes)
	}

	return xc.Address(""), fmt.Errorf("invalid algorithm: %s", ab.alghoritm)
}

func hash(ingest []byte, hashSize int) ([]byte, error) {
	hash, err := blake2b.New(hashSize, nil)
	if err != nil {
		return nil, err
	}

	if _, err = hash.Write(ingest); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

// Filecoin address prefix is set to "f" for mainnet and "t" for testnet
func (ab AddressBuilder) getPrefix() (string, error) {
	switch ab.network {
	case "mainnet":
		return "f", nil
	case "testnet":
		return "t", nil
	default:
		return "", fmt.Errorf("invalid network: %s", ab.network)
	}
}

func ProtocolIdToBytes(rawAddress string) ([]byte, error) {
	if len(rawAddress) > protocolIdRawMaxLength {
		return nil, fmt.Errorf("invalid address length: %d. Expected: %v", len(rawAddress), protocolIdRawMaxLength)
	}
	id, err := strconv.ParseUint(rawAddress, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse id: %w", err)
	}
	return toBytes(ProtocolId, toUvarint(id))

}

// Prepare address for message serialization
func AddressToBytes(addr string) ([]byte, error) {
	if len(addr) == 0 {
		return nil, errors.New("empty address")
	}
	var protocol byte
	switch addr[1] {
	case '0':
		protocol = ProtocolId
	case '1':
		protocol = ProtocolSecp256k1
	case '2':
		protocol = ProtocolActor
	case '3':
		protocol = ProtocolBls
	// Delegate transactions require special transaction handling.
	// For example 410 is an EVM address space, which
	// is not supported by this implementation.
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", addr[1])

	}

	// Address without the network prefix and protocol
	rawAddress := addr[2:]
	if protocol == ProtocolId {
		return ProtocolIdToBytes(rawAddress)
	}

	payloadcksm, err := Encoding.WithPadding(-1).DecodeString(rawAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to decode address: %w", err)
	}
	if len(payloadcksm) < ChecksumHashLength {
		return nil, fmt.Errorf("invalid checksum length: %d. Expected: %d", len(payloadcksm), ChecksumHashLength)
	}

	payload := payloadcksm[:len(payloadcksm)-ChecksumHashLength]
	checksum := payloadcksm[len(payloadcksm)-ChecksumHashLength:]

	if protocol == ProtocolSecp256k1 || protocol == ProtocolActor {
		if len(payload) != 20 {
			return nil, fmt.Errorf("invalid payload length: %d. Expected: 20", len(payload))
		}
	}

	if !validateChecksum(append([]byte{protocol}, payload...), checksum) {
		return nil, errors.New("invalid checksum")
	}

	return toBytes(protocol, payload)
}

func toBytes(protocol byte, payload []byte) ([]byte, error) {
	switch protocol {
	case ProtocolId:
		_, n, err := fromUvarint(payload)
		if err != nil {
			return nil, fmt.Errorf("failed from uvariant: %w", err)
		}
		if n != len(payload) {
			return nil, fmt.Errorf("invalid payload length: %d. Expected: %d", n, len(payload))
		}
	case ProtocolSecp256k1, ProtocolActor:
		if len(payload) != 20 {
			return nil, fmt.Errorf("invalid payload length: %d. Expected: 20", len(payload))
		}
	case ProtocolBls:
		if len(payload) != 48 {
			return nil, fmt.Errorf("invalid payload length: %d. Expected: 48", len(payload))
		}
	case ProtocolDelegated:
		namespace, n, err := fromUvarint(payload)
		if err != nil {
			return nil, fmt.Errorf("failed from uvariant: %w", err)
		}
		if namespace > math.MaxInt64 {
			return nil, fmt.Errorf("invalid namespace: %d. Expected: namespace < %d", namespace, math.MaxInt64)
		}
		if len(payload)-n > MaxSubaddressLen {
			return nil, fmt.Errorf("invalid subaddress length: %d. Expected: subaddress < %d", len(payload)-n, MaxSubaddressLen)
		}
	default:
		return nil, fmt.Errorf("unsupported protocol: %d", protocol)
	}

	expectedLength := 1 + len(payload)
	buf := make([]byte, expectedLength)
	buf[0] = protocol
	copy(buf[1:], payload)

	return buf, nil
}

// fromUvarint reads an unsigned varint from the beginning of buf, returns the
// varint, and the number of bytes read.
func fromUvarint(buf []byte) (uint64, int, error) {
	// Modified from the go standard library. Copyright the Go Authors and
	// released under the BSD License.
	var x uint64
	var s uint
	for i, b := range buf {
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, 0, errors.New("varints larger than uint64 not yet supported")
			} else if b == 0 && s > 0 {
				return 0, 0, errors.New("varint not minimally encoded")
			}
			return x | uint64(b)<<s, i + 1, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, 0, errors.New("varints malformed, could not reach the end")
}

func toUvarint(num uint64) []byte {
	buf := make([]byte, uvarintSize(num))
	n := binary.PutUvarint(buf, uint64(num))
	return buf[:n]
}

func uvarintSize(num uint64) int {
	bits := bits.Len64(num)
	q, r := bits/7, bits%7
	size := q
	if r > 0 || size == 0 {
		size++
	}
	return size
}

func validateChecksum(ingest, expect []byte) bool {
	digest, err := hash(ingest, checksumHashSize)
	if err != nil {
		return false
	}
	return bytes.Equal(digest, expect)
}
