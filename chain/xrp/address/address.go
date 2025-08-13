package address

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	"golang.org/x/crypto/ripemd160"
)

// AddressBuilder for XRP
type AddressBuilder struct {
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new XRP AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{}, nil
}

// Custom base58 dictionary for XRP
const xrpBase58Alphabet = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"

// EncodeBase58 encodes a byte slice into a base58 string.
func EncodeBase58(input []byte) string {
	var result []byte

	intVal := new(big.Int).SetBytes(input)

	base := big.NewInt(int64(len(xrpBase58Alphabet)))
	zero := big.NewInt(0)
	mod := &big.Int{}

	for intVal.Cmp(zero) > 0 {
		intVal.DivMod(intVal, base, mod)
		result = append(result, xrpBase58Alphabet[mod.Int64()])
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	for _, b := range input {
		if b == 0x00 {
			result = append([]byte{xrpBase58Alphabet[0]}, result...)
		} else {
			break
		}
	}

	return string(result)
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	// Ensure the public key is either 33 bytes (secp256k1) or 32 bytes (Ed25519).
	if len(publicKeyBytes) != 33 && len(publicKeyBytes) != 32 {
		return "", fmt.Errorf("invalid public key length")
	}

	// For Ed25519 keys, prefix the key with 0xED.
	if len(publicKeyBytes) == 32 {
		publicKeyBytes = append([]byte{0xED}, publicKeyBytes...)
	}

	// Calculate RIPEMD160 for SHA256 of publicKey.
	sha256Hash := sha256.Sum256(publicKeyBytes)
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])
	accountID := ripemd160Hasher.Sum(nil)

	// Calculate the checksum SHA256 hash of the SHA256 hash of the payload and take the first 4 bytes.
	addressTypePrefix := []byte{0x00} // This is the type prefix for classic XRP addresses.
	payload := append(addressTypePrefix, accountID...)
	firstSHA256 := sha256.Sum256(payload)
	secondSHA256 := sha256.Sum256(firstSHA256[:])
	checksum := secondSHA256[:4]

	// Concatenate the payload and checksum, and encode with custom base58 dictionary.
	dataToEncode := append(payload, checksum...)
	address := EncodeBase58(dataToEncode)

	return xc.Address(address), nil
}
