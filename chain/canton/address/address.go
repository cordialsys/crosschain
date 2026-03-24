package address

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

// AddressBuilder for Canton
type AddressBuilder struct{}

var _ xc.AddressBuilder = AddressBuilder{}

func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// GetAddressFromPublicKey returns a Canton party ID from a raw Ed25519 public key (32 bytes).
//
// Canton party IDs have the form:  <name>::<fingerprint>
//
// Where:
//   - <name>        = hex-encoded public key (64 hex chars)
//   - <fingerprint> = "1220" + hex(SHA-256(purposeBytes || rawPubKey))
//
// The purpose prefix is big-endian uint32(12), matching Canton's internal
// HashPurpose.PublicKeyFingerprint (id=12) in Hash.digest().
//
// The "1220" multihash prefix encodes:
//   - 0x12 = SHA-256 algorithm code (varint)
//   - 0x20 = 32-byte digest length (varint)
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	return GetAddressFromPublicKey(publicKeyBytes)
}

func (ab AddressBuilder) AddressRegistrationRequired(address xc.Address) bool {
	return true
}

func GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	if len(publicKeyBytes) != 32 {
		return "", fmt.Errorf("invalid ed25519 public key length: expected 32 bytes, got %d", len(publicKeyBytes))
	}
	fingerprint := computeFingerprint(publicKeyBytes)
	name := hex.EncodeToString(publicKeyBytes)
	addr := xc.Address(name + "::" + fingerprint)

	return addr, nil
}

// computeFingerprint returns the Canton key fingerprint for a raw Ed25519 public key.
//
//	fingerprint = "1220" + hex(SHA-256(bigEndianUint32(12) || rawPubKey))
func computeFingerprint(rawPubKey []byte) string {
	// HashPurpose.PublicKeyFingerprint id=12 encoded as big-endian int32 (4 bytes)
	var purposeBytes [4]byte
	binary.BigEndian.PutUint32(purposeBytes[:], 12)

	h := sha256.New()
	h.Write(purposeBytes[:])
	h.Write(rawPubKey)
	digest := h.Sum(nil)

	// Multihash: varint(0x12=SHA-256) || varint(0x20=32) || digest
	return "1220" + hex.EncodeToString(digest)
}

// ParsePartyID splits a Canton party ID into its name and fingerprint components.
// Expected format: "<name>::<fingerprint>" where fingerprint starts with "1220".
func ParsePartyID(addr xc.Address) (name string, fingerprint string, err error) {
	s := string(addr)
	idx := strings.Index(s, "::")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid Canton party ID %q: missing '::' separator", s)
	}
	name = s[:idx]
	fingerprint = s[idx+2:]
	if len(fingerprint) < 4 || fingerprint[:4] != "1220" {
		return "", "", fmt.Errorf("invalid Canton fingerprint %q: must start with '1220' (SHA-256 multihash prefix)", fingerprint)
	}
	return name, fingerprint, nil
}
