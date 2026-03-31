package crypto

import (
	"fmt"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/sha3"
)

const (
	// Monero mainnet address prefix
	MainnetAddressPrefix byte = 0x12 // 18
	// Monero mainnet integrated address prefix
	MainnetIntegratedPrefix byte = 0x13 // 19
	// Monero mainnet subaddress prefix
	MainnetSubaddressPrefix byte = 0x2a // 42
)

// Keccak256 computes the Keccak-256 hash of data (NOT SHA3-256; Monero uses the pre-NIST Keccak)
func Keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// ScalarReduce reduces a 32-byte value modulo the ed25519 group order L.
// Uses Monero's sc_reduce32 (32-byte input, not 64-byte).
func ScalarReduce(input []byte) []byte {
	return ScReduce32(input)
}

// DeriveViewKey derives the private view key from the private spend key.
// In Monero: view_key = Keccak256(spend_key) mod L
func DeriveViewKey(privateSpendKey []byte) []byte {
	hash := Keccak256(privateSpendKey)
	return ScalarReduce(hash)
}

// PublicFromPrivate derives the ed25519 public key from a Monero private key scalar.
// In Monero, the private key is a scalar s and the public key is s*G.
func PublicFromPrivate(privateKey []byte) ([]byte, error) {
	sc, err := edwards25519.NewScalar().SetCanonicalBytes(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key scalar: %w", err)
	}
	pub := edwards25519.NewGeneratorPoint().ScalarBaseMult(sc)
	return pub.Bytes(), nil
}

// GenerateAddress creates a Monero address from public spend key and public view key.
// Address = base58(prefix || pub_spend || pub_view || checksum)
// where checksum = first 4 bytes of Keccak256(prefix || pub_spend || pub_view)
func GenerateAddress(publicSpendKey, publicViewKey []byte) string {
	return GenerateAddressWithPrefix(MainnetAddressPrefix, publicSpendKey, publicViewKey)
}

// GenerateAddressWithPrefix creates a Monero address with a given prefix byte.
func GenerateAddressWithPrefix(prefix byte, publicSpendKey, publicViewKey []byte) string {
	data := make([]byte, 0, 1+32+32+4)
	data = append(data, prefix)
	data = append(data, publicSpendKey...)
	data = append(data, publicViewKey...)

	checksum := Keccak256(data)[:4]
	data = append(data, checksum...)

	return MoneroBase58Encode(data)
}

// DecodeAddress decodes a Monero address and returns (prefix, publicSpendKey, publicViewKey, error)
func DecodeAddress(address string) (byte, []byte, []byte, error) {
	decoded, err := MoneroBase58Decode(address)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to decode address: %w", err)
	}
	if len(decoded) != 69 {
		return 0, nil, nil, fmt.Errorf("invalid address length: got %d, expected 69", len(decoded))
	}

	prefix := decoded[0]
	pubSpend := decoded[1:33]
	pubView := decoded[33:65]
	checksum := decoded[65:69]

	// Verify checksum
	expectedChecksum := Keccak256(decoded[:65])[:4]
	for i := 0; i < 4; i++ {
		if checksum[i] != expectedChecksum[i] {
			return 0, nil, nil, fmt.Errorf("invalid address checksum")
		}
	}

	return prefix, pubSpend, pubView, nil
}

// DeriveKeysFromSpend derives the full Monero key set from a private spend key:
// Returns (privateSpendKey, privateViewKey, publicSpendKey, publicViewKey, error)
func DeriveKeysFromSpend(privateSpendKey []byte) (privSpend, privView, pubSpend, pubView []byte, err error) {
	// Ensure spend key is properly reduced
	privSpend = ScalarReduce(privateSpendKey)
	privView = DeriveViewKey(privSpend)

	pubSpend, err = PublicFromPrivate(privSpend)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to derive public spend key: %w", err)
	}

	pubView, err = PublicFromPrivate(privView)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to derive public view key: %w", err)
	}

	return privSpend, privView, pubSpend, pubView, nil
}
