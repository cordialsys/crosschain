package address

import (
	"crypto/sha256"
	"crypto/x509/pkix"
	"fmt"
	"hash/crc32"

	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
)

var (
	DefaultPrincipalSubaccount [32]byte
)

// AddressBuilder for Template
type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

type publicKeyInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	var der, err = asn1.Marshal(publicKeyInfo{
		Algorithm: pkix.AlgorithmIdentifier{
			Algorithm: asn1.ObjectIdentifier{1, 3, 101, 112},
		},
		PublicKey: asn1.BitString{
			BitLength: len(publicKeyBytes) * 8,
			Bytes:     publicKeyBytes,
		},
	})
	if err != nil {
		return xc.Address(""), fmt.Errorf("failed to marshal public key: %w", err)
	}

	principal := principalFromDerPublicKey(der)
	accountId := newAccountId(principal)
	address := hex.EncodeToString(accountId)
	return xc.Address(address), nil
}

func principalFromDerPublicKey(der []byte) []byte {
	hash := sha256.Sum224(der)
	return append(hash[:], 0x02)
}

func newAccountId(principal []byte) []byte {
	h := sha256.New224()
	h.Write([]byte("\x0Aaccount-id"))
	h.Write(principal)
	h.Write(DefaultPrincipalSubaccount[:])
	bs := h.Sum(nil)

	var accountId [28]byte
	copy(accountId[:], bs)

	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(accountId[:]))

	return append(crc, accountId[:]...)
}
