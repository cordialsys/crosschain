package address

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"golang.org/x/crypto/ripemd160"
)

// AddressBuilder for Template
type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	// Parse and serialize as 33 byte compressed public key format.
	if len(publicKeyBytes) == 32 {
		withCompressedHeader := make([]byte, 33)
		withCompressedHeader[0] = 0x02
		copy(withCompressedHeader[1:], publicKeyBytes)
		publicKeyBytes = withCompressedHeader
	}
	pubkey, err := btcec.ParsePubKey(publicKeyBytes)
	if err != nil {
		return "", err
	}

	hash := Ripemd160Checksum(pubkey.SerializeCompressed(), ecc.CurveK1)

	pubkeyAndChecksum := make([]byte, len(publicKeyBytes)+4)
	copy(pubkeyAndChecksum, publicKeyBytes)
	copy(pubkeyAndChecksum[len(publicKeyBytes):], hash[:4])

	address := base58.Encode(pubkeyAndChecksum)
	prefixed := "EOS" + address

	return xc.Address(prefixed), nil
}

func Ripemd160Checksum(in []byte, curve ecc.CurveID) []byte {
	h := ripemd160.New()
	_, _ = h.Write(in) // this implementation has no error path

	if curve != ecc.CurveK1 {
		_, _ = h.Write([]byte(curve.String()))
	}

	sum := h.Sum(nil)
	return sum[:4]
}
