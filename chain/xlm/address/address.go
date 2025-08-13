package address

import (
	base32 "encoding/base32"
	"encoding/binary"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

// base32 encodes to G
const VersionByte byte = 6 << 3

type AddressBuilder struct{}

var _ xc.AddressBuilder = AddressBuilder{}

func NewAddressBuilder(cfg *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey generates an Address from a given public key.
// The method takes a public key in the form of a byte slice and returns a corresponding Address
// after encoding it with a version byte and a checksum. The public key must be 32 bytes long.
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) != 32 {
		return xc.Address(""), fmt.Errorf("expected public key length 32, got %v", len(publicKeyBytes))
	}

	// Allocate space for publicKeyBytes + versionByte(1) + checksum(2)
	fullKeyLen := 32 + 1 + 2
	raw := make([]byte, fullKeyLen)
	raw[0] = VersionByte

	// Add version byte prefix
	copy(raw[1:], publicKeyBytes)

	// Calculate checksum - omit padding zeros
	checksum := Checksum(raw[:1+32])

	// Put the checksum at the end of our array
	lenWithVersion := 32 + 1
	binary.LittleEndian.PutUint16(raw[lenWithVersion:], checksum)

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	return xc.Address(encoded), nil
}
