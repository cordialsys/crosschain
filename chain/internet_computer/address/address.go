package address

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509/pkix"
	"fmt"
	"hash/crc32"

	"encoding/asn1"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
)

const (
	FormatIcp   = "icp"
	FormatIcrc1 = "icrc1"
)

var (
	// AnonymousPrincipal is used for anynymous requests. It can query/call without
	// a signature.
	AnonymousPrincipal = Principal{Raw: []byte{0x04}}
	Encoding           = base32.StdEncoding.WithPadding(base32.NoPadding)
	ICRC1Account       = "icrc1"
)

// AddressBuilder for InternetComputerProtocol
type AddressBuilder struct {
	Format xc.AddressFormat
}

type AccountId []byte

var _ xc.AddressBuilder = AddressBuilder{}
var _ xc.AddressBuilderWithFormats = AddressBuilder{}

// NewAddressBuilder creates a new InternetComputerProtocol AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	opts, err := xcaddress.NewAddressOptions(options...)
	if err != nil {
		return AddressBuilder{}, err
	}

	format, ok := opts.GetFormat()
	if ok {
		if format != "" && format != FormatIcp && format != FormatIcrc1 {
			return nil, fmt.Errorf(
				"unsupported format: %s, expected: ['icp', 'icrc1'] - default: 'icp'",
				format,
			)
		}
	}
	return AddressBuilder{
		Format: format,
	}, nil
}

type publicKeyInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

func (ab AddressBuilder) GetSignatureAlgorithm() xc.SignatureType {
	return xc.ICP.Driver().SignatureAlgorithms()[0]
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	principal, err := PrincipalFromPublicKey(publicKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to get principal from public key: %w", err)
	}

	// ICP account format differs from ICRC1
	if ab.Format == FormatIcp || ab.Format == "" {
		accountId := NewAccountId(principal)
		address := accountId.Encode()
		return xc.Address(address), nil
	} else {
		pk := ed25519.PublicKey(publicKeyBytes)
		id := Ed25519Identity{
			PublicKey: pk,
		}
		principal, err := id.Principal()
		if err != nil {
			return xc.Address(""), fmt.Errorf("failed to create principal: %w", err)
		}
		return xc.Address(principal.Encode()), nil
	}
}

func DerEncodePublicKey(publicKey []byte) ([]byte, error) {
	return asn1.Marshal(publicKeyInfo{
		Algorithm: pkix.AlgorithmIdentifier{
			Algorithm: asn1.ObjectIdentifier{1, 3, 101, 112},
		},
		PublicKey: asn1.BitString{
			BitLength: len(publicKey) * 8,
			Bytes:     publicKey,
		},
	})
}

func NewAccountId(principal []byte) AccountId {
	h := sha256.New224()
	h.Write([]byte("\x0Aaccount-id"))
	h.Write(principal)
	h.Write(DefaultPrincipalSubaccount[:])
	bs := h.Sum(nil)

	var accountId [28]byte
	copy(accountId[:], bs)

	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(accountId[:]))

	return AccountId(append(crc, accountId[:]...))
}

func (accountId AccountId) Encode() string {
	return hex.EncodeToString(accountId)
}
