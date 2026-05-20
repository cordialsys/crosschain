package crypto

// Pure Go implementations of Monero crypto primitives.
// These replace the CGO functions in cref/monero_crypto.go.

import (
	"encoding/hex"

	"filippo.io/edwards25519"
)

// hPointHex is the compressed Edwards representation of Monero's H point.
const hPointHex = "8b655970153799af2aeadc9ff1add0ea6c7251d54154cfa92c173a0dd39c1f94"

// scReduce32PureGo reduces a 32-byte value mod the ed25519 group order L.
// L = 2^252 + 27742317777372353535851937790883648493
//
// For values < L, this is identity. For values >= L, subtract L.
// This matches Monero's sc_reduce32.
func ScReduce32PureGo(s []byte) []byte {
	if len(s) != 32 {
		buf := make([]byte, 32)
		copy(buf, s)
		s = buf
	}

	// Try to load as a canonical scalar (< L)
	result, err := edwards25519.NewScalar().SetCanonicalBytes(s)
	if err == nil {
		return result.Bytes()
	}

	// Value >= L: use SetUniformBytes with 64-byte input (padded)
	// This reduces mod L correctly for any 256-bit input
	wide := make([]byte, 64)
	copy(wide, s)
	result, err = edwards25519.NewScalar().SetUniformBytes(wide)
	if err != nil {
		// Should never happen
		return make([]byte, 32)
	}
	return result.Bytes()
}

// generateKeyDerivationPureGo computes D = 8 * secret * public (cofactor ECDH).
func GenerateKeyDerivationPureGo(pub, sec []byte) ([]byte, error) {
	pubPoint, err := edwards25519.NewIdentityPoint().SetBytes(pub)
	if err != nil {
		return nil, err
	}

	secScalar, err := edwards25519.NewScalar().SetCanonicalBytes(sec)
	if err != nil {
		return nil, err
	}

	// D = sec * pub
	D := edwards25519.NewIdentityPoint().ScalarMult(secScalar, pubPoint)

	// Multiply by cofactor 8 (3 doublings)
	D2 := edwards25519.NewIdentityPoint().Add(D, D)
	D4 := edwards25519.NewIdentityPoint().Add(D2, D2)
	D8 := edwards25519.NewIdentityPoint().Add(D4, D4)

	return D8.Bytes(), nil
}

// GenerateKeyImagePureGo computes I = secret * hash_to_ec(public).
// keccakPub should be Keccak256(public_key), sec is the secret scalar.
func GenerateKeyImagePureGo(keccakPub, sec []byte) []byte {
	// hash_to_ec without re-hashing: Elligator map + cofactor on pre-hashed input
	point := geFromfeFrombytesVartime(keccakPub)
	p2 := edwards25519.NewIdentityPoint().Add(point, point)
	p4 := edwards25519.NewIdentityPoint().Add(p2, p2)
	p8 := edwards25519.NewIdentityPoint().Add(p4, p4)
	hp := p8.Bytes()

	hpPoint, err := edwards25519.NewIdentityPoint().SetBytes(hp)
	if err != nil {
		return make([]byte, 32)
	}

	secScalar, err := edwards25519.NewScalar().SetCanonicalBytes(sec)
	if err != nil {
		return make([]byte, 32)
	}

	// I = sec * H_p
	I := edwards25519.NewIdentityPoint().ScalarMult(secScalar, hpPoint)
	return I.Bytes()
}

// getHPureGo returns the precomputed H generator point.
func GetHPureGo() []byte {
	b, _ := hex.DecodeString(hPointHex)
	return b
}
