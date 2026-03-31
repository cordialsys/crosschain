package crypto

import (
	"encoding/hex"
	"fmt"

	"filippo.io/edwards25519"
)

// CheckTxOutputOwnership checks if a transaction output belongs to the given view key and public spend key.
// For each output in a Monero transaction:
//   - tx_pub_key (R) is the transaction public key
//   - output_key (P) is the one-time output public key
//   - We check: P == H_s(view_key * R || output_index) * G + public_spend_key
//
// Returns true if the output belongs to us, along with the derived key image preimage.
func CheckTxOutputOwnership(
	txPubKeyHex string,
	outputKeyHex string,
	outputIndex uint64,
	privateViewKey []byte,
	publicSpendKey []byte,
) (bool, error) {
	txPubKeyBytes, err := hex.DecodeString(txPubKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid tx pub key hex: %w", err)
	}
	outputKeyBytes, err := hex.DecodeString(outputKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid output key hex: %w", err)
	}

	// R = tx public key (point)
	R, err := edwards25519.NewIdentityPoint().SetBytes(txPubKeyBytes)
	if err != nil {
		return false, fmt.Errorf("invalid tx public key point: %w", err)
	}

	// a = private view key (scalar)
	a, err := edwards25519.NewScalar().SetCanonicalBytes(privateViewKey)
	if err != nil {
		return false, fmt.Errorf("invalid view key scalar: %w", err)
	}

	// D = a * R (shared secret point)
	D := edwards25519.NewIdentityPoint().ScalarMult(a, R)

	// H_s(D || output_index) - hash to scalar
	derivationData := D.Bytes()
	derivationData = append(derivationData, varintEncode(outputIndex)...)
	scalarHash := Keccak256(derivationData)
	hs := ScalarReduce(scalarHash)

	// H_s * G
	hsScalar, err := edwards25519.NewScalar().SetCanonicalBytes(hs)
	if err != nil {
		return false, fmt.Errorf("invalid derived scalar: %w", err)
	}
	hsG := edwards25519.NewGeneratorPoint().ScalarBaseMult(hsScalar)

	// B = public spend key (point)
	B, err := edwards25519.NewIdentityPoint().SetBytes(publicSpendKey)
	if err != nil {
		return false, fmt.Errorf("invalid public spend key point: %w", err)
	}

	// Expected P' = H_s * G + B
	expectedP := edwards25519.NewIdentityPoint().Add(hsG, B)

	// Compare with actual output key
	P, err := edwards25519.NewIdentityPoint().SetBytes(outputKeyBytes)
	if err != nil {
		return false, fmt.Errorf("invalid output key point: %w", err)
	}

	return expectedP.Equal(P) == 1, nil
}

// varintEncode encodes a uint64 as a Monero-style varint
func varintEncode(val uint64) []byte {
	var result []byte
	for val >= 0x80 {
		result = append(result, byte(val&0x7f)|0x80)
		val >>= 7
	}
	result = append(result, byte(val))
	return result
}
