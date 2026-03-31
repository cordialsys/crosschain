package crypto

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"filippo.io/edwards25519"
	"github.com/cordialsys/crosschain/chain/monero/crypto/cref"
)

// GenerateKeyDerivation computes D = 8 * viewKey * txPubKey
// Uses Monero's exact C implementation for correctness.
func GenerateKeyDerivation(txPubKey []byte, privateViewKey []byte) ([]byte, error) {
	if len(txPubKey) != 32 || len(privateViewKey) != 32 {
		return nil, fmt.Errorf("invalid key lengths: pub=%d, sec=%d", len(txPubKey), len(privateViewKey))
	}
	result := cref.GenerateKeyDerivation(txPubKey, privateViewKey)
	return result[:], nil
}

// DerivationToScalar computes s = H_s(derivation || varint(outputIndex))
// where H_s is Keccak256 followed by sc_reduce32.
func DerivationToScalar(derivation []byte, outputIndex uint64) ([]byte, error) {
	data := make([]byte, 0, 32+10)
	data = append(data, derivation...)
	data = append(data, varintEncode(outputIndex)...)

	hash := Keccak256(data)
	return ScReduce32(hash), nil
}

// DerivePublicKey computes P' = s*G + publicSpendKey
// Used to check if an output belongs to us.
func DerivePublicKey(scalar []byte, publicSpendKey []byte) ([]byte, error) {
	s, err := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	if err != nil {
		return nil, fmt.Errorf("invalid scalar: %w", err)
	}

	B, err := edwards25519.NewIdentityPoint().SetBytes(publicSpendKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public spend key: %w", err)
	}

	// s * G
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(s)

	// P' = s*G + B
	result := edwards25519.NewIdentityPoint().Add(sG, B)
	return result.Bytes(), nil
}

// DecryptAmount decrypts a RingCT encrypted amount using the derivation scalar.
// For modern Monero (Bulletproofs+), the amount is XORed with H_s("amount" || scalar).
func DecryptAmount(encryptedAmountHex string, scalar []byte) (uint64, error) {
	encryptedAmount, err := hex.DecodeString(encryptedAmountHex)
	if err != nil {
		return 0, fmt.Errorf("invalid encrypted amount hex: %w", err)
	}

	if len(encryptedAmount) != 8 {
		return 0, fmt.Errorf("encrypted amount must be 8 bytes, got %d", len(encryptedAmount))
	}

	// amount_key = H_s("amount" || scalar)
	data := make([]byte, 0, 7+32)
	data = append(data, []byte("amount")...)
	data = append(data, scalar...)

	amountKey := Keccak256(data)
	// No need to reduce - we just XOR with first 8 bytes

	// Decrypt: amount = encrypted_amount XOR first_8_bytes(amount_key)
	decrypted := make([]byte, 8)
	for i := 0; i < 8; i++ {
		decrypted[i] = encryptedAmount[i] ^ amountKey[i]
	}

	// Interpret as little-endian uint64
	amount := binary.LittleEndian.Uint64(decrypted)
	return amount, nil
}

// ParseTxPubKey extracts the transaction public key from the tx extra field.
// The extra field format: tag(1 byte) followed by data.
// Tag 0x01 = tx public key (32 bytes follow)
// Tag 0x02 = extra nonce (variable length)
// Tag 0x04 = additional public keys
func ParseTxPubKey(extra []byte) ([]byte, error) {
	for i := 0; i < len(extra); {
		if i >= len(extra) {
			break
		}
		tag := extra[i]
		i++

		switch tag {
		case 0x01: // TX_EXTRA_TAG_PUBKEY
			if i+32 > len(extra) {
				return nil, fmt.Errorf("extra field too short for tx pub key")
			}
			return extra[i : i+32], nil
		case 0x02: // TX_EXTRA_NONCE
			if i >= len(extra) {
				return nil, fmt.Errorf("extra field too short for nonce length")
			}
			nonceLen := int(extra[i])
			i += 1 + nonceLen
		case 0x04: // TX_EXTRA_TAG_ADDITIONAL_PUBKEYS
			if i >= len(extra) {
				return nil, fmt.Errorf("extra field too short for additional keys count")
			}
			count := int(extra[i])
			i += 1 + count*32
		case 0xDE: // TX_EXTRA_MYSTERIOUS_MINERGATE_TAG
			if i >= len(extra) {
				break
			}
			// Variable length - read varint
			size, bytesRead := varintDecode(extra[i:])
			i += bytesRead + int(size)
		default:
			// Unknown tag, try to continue
			// For padding (0x00), just skip
			continue
		}
	}
	return nil, fmt.Errorf("tx public key not found in extra field")
}

// varintDecode decodes a Monero-style varint, returning (value, bytesRead)
func varintDecode(data []byte) (uint64, int) {
	var val uint64
	var shift uint
	for i, b := range data {
		val |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return val, i + 1
		}
		shift += 7
		if shift >= 64 {
			break
		}
	}
	return val, len(data)
}

// ScanOutput checks if a transaction output belongs to the wallet identified by
// (privateViewKey, publicSpendKey) and returns the decrypted amount if it does.
func ScanOutput(
	txPubKey []byte,
	outputIndex uint64,
	outputKeyHex string,
	encryptedAmountHex string,
	privateViewKey []byte,
	publicSpendKey []byte,
) (owned bool, amount uint64, err error) {
	// 1. Generate key derivation: D = 8 * viewKey * txPubKey
	derivation, err := GenerateKeyDerivation(txPubKey, privateViewKey)
	if err != nil {
		return false, 0, fmt.Errorf("key derivation failed: %w", err)
	}

	// 2. Compute scalar: s = H_s(D || outputIndex)
	scalar, err := DerivationToScalar(derivation, outputIndex)
	if err != nil {
		return false, 0, fmt.Errorf("derivation to scalar failed: %w", err)
	}

	// 3. Compute expected output key: P' = s*G + publicSpendKey
	expectedKey, err := DerivePublicKey(scalar, publicSpendKey)
	if err != nil {
		return false, 0, fmt.Errorf("derive public key failed: %w", err)
	}

	// 4. Compare with actual output key
	outputKeyBytes, err := hex.DecodeString(outputKeyHex)
	if err != nil {
		return false, 0, fmt.Errorf("invalid output key hex: %w", err)
	}

	if len(expectedKey) != len(outputKeyBytes) {
		return false, 0, nil
	}
	match := true
	for i := range expectedKey {
		if expectedKey[i] != outputKeyBytes[i] {
			match = false
			break
		}
	}

	if !match {
		return false, 0, nil
	}

	// 5. Output belongs to us - decrypt amount
	if encryptedAmountHex != "" {
		amount, err = DecryptAmount(encryptedAmountHex, scalar)
		if err != nil {
			return true, 0, fmt.Errorf("amount decryption failed: %w", err)
		}
	}

	return true, amount, nil
}
