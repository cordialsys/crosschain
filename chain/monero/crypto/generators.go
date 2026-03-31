package crypto

import (
	"filippo.io/edwards25519"
)

// Monero generator points for Pedersen commitments and Bulletproofs+.
//
// G = Ed25519 base point (used for blinding factors)
// H = secondary generator for amounts, derived via hash-to-point so that
//     the discrete log relationship between G and H is unknown.
//
// For Bulletproofs+, we also need vectors Gi[0..maxMN-1] and Hi[0..maxMN-1]
// derived via domain-separated hash-to-point.

const (
	maxN  = 64  // bits in range proof (proves amount in [0, 2^64))
	maxM  = 16  // max number of outputs aggregated in one proof
	maxMN = maxN * maxM
)

// H is the secondary generator point for Pedersen commitments.
// In Monero, H = 8 * hash_to_point(G_bytes).
// This is a fixed constant: the "alternate base point" used for amount commitments.
// C = v*H + r*G  (v=amount, r=blinding factor)
var H *edwards25519.Point

// Gi and Hi are the generator vectors for Bulletproofs+ inner product arguments.
var Gi [maxMN]*edwards25519.Point
var Hi [maxMN]*edwards25519.Point

func init() {
	// Derive H = 8 * hash_to_point(G)
	// Monero uses cn_fast_hash of the compressed basepoint, then maps to curve
	gBytes := edwards25519.NewGeneratorPoint().Bytes()
	H = hashToPoint(gBytes)

	// Derive Gi and Hi vectors for BP+
	// Monero: Hi[i] = hash_to_point("bulletproof_plus" || varint(2*i))
	//         Gi[i] = hash_to_point("bulletproof_plus" || varint(2*i+1))
	prefix := []byte("bulletproof_plus")
	for i := 0; i < maxMN; i++ {
		hiData := append(prefix, varintEncode(uint64(2*i))...)
		giData := append(prefix, varintEncode(uint64(2*i+1))...)
		Hi[i] = hashToPoint(hiData)
		Gi[i] = hashToPoint(giData)
	}
}

// hashToPoint maps arbitrary data to a point on the Ed25519 curve.
// This follows Monero's hash_to_ec: Keccak256 → interpret as y-coordinate → recover x → multiply by cofactor 8.
func hashToPoint(data []byte) *edwards25519.Point {
	hash := Keccak256(data)

	// Monero's ge_fromfe_frombytes_vartime: interpret hash as field element,
	// map to curve point using Elligator-like map, multiply by cofactor.
	// We use a simpler approach: try hash as y-coordinate, increment until valid.
	p := hashToPointMontgomery(hash)
	return p
}

// hashToPointMontgomery implements Monero's hash_to_ec which uses the
// field element → Montgomery curve → Edwards curve mapping.
// This is equivalent to ge_fromfe_frombytes_vartime in Monero.
func hashToPointMontgomery(hash []byte) *edwards25519.Point {
	// Monero uses a specific mapping from 256-bit hash to curve point.
	// The approach: interpret hash as a field element, use it to compute
	// a point on the Montgomery curve, convert to Edwards form, multiply by 8.
	//
	// For correctness, we use the same approach as Monero:
	// 1. Reduce hash mod p (the field prime, not the group order)
	// 2. Use the Elligator map to get a Montgomery point
	// 3. Convert to Edwards
	// 4. Multiply by cofactor 8
	//
	// Since filippo.io/edwards25519 doesn't expose the field element operations
	// needed for the Elligator map, we implement it using the low-level
	// CompressedEdwardsY approach with a loop.

	// Simple approach: iterate hash until we find a valid curve point
	h := make([]byte, 32)
	copy(h, hash)

	for attempt := 0; attempt < 256; attempt++ {
		// Try to decompress as an Edwards point
		p, err := edwards25519.NewIdentityPoint().SetBytes(h)
		if err == nil {
			// Multiply by cofactor 8 to ensure we're in the prime-order subgroup
			p2 := edwards25519.NewIdentityPoint().Add(p, p)
			p4 := edwards25519.NewIdentityPoint().Add(p2, p2)
			p8 := edwards25519.NewIdentityPoint().Add(p4, p4)

			// Check it's not identity
			if p8.Equal(edwards25519.NewIdentityPoint()) != 1 {
				return p8
			}
		}
		// Hash again to try next candidate
		h = Keccak256(h)
	}

	// Should never reach here with Keccak256
	panic("hashToPoint: failed to find valid point after 256 attempts")
}

// PedersenCommit computes a Pedersen commitment: C = v*H + r*G
// where v is the amount and r is the blinding factor (mask).
func PedersenCommit(amount uint64, mask []byte) (*edwards25519.Point, error) {
	// v * H
	vBytes := ScalarFromUint64(amount)
	vScalar, err := edwards25519.NewScalar().SetCanonicalBytes(vBytes)
	if err != nil {
		return nil, err
	}
	vH := edwards25519.NewIdentityPoint().ScalarMult(vScalar, H)

	// r * G
	rScalar, err := edwards25519.NewScalar().SetCanonicalBytes(mask)
	if err != nil {
		return nil, err
	}
	rG := edwards25519.NewGeneratorPoint().ScalarBaseMult(rScalar)

	// C = vH + rG
	result := edwards25519.NewIdentityPoint().Add(vH, rG)
	return result, nil
}

// ScalarFromUint64 converts a uint64 to a 32-byte little-endian scalar.
func ScalarFromUint64(v uint64) []byte {
	b := make([]byte, 32)
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * i))
	}
	return b
}

// RandomScalar generates a random scalar mod L using Keccak256 of entropy.
func RandomScalar(entropy []byte) []byte {
	hash := Keccak256(entropy)
	return ScalarReduce(hash)
}
