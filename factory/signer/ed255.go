package signer

import (
	"crypto/rand"
	"crypto/sha512"
	"fmt"

	"filippo.io/edwards25519"
)

// Normally a ed25519 private key is in a 64 byte {seed, public-key} format.
// During signing, the seed is hashed to derive the actual scalar.  This is not
// very compatible with MPC algorithms, so we provide this alternative signing implementation.
// This will perform the signing using the "naked" scalar -- what would be part of the hashed "seed".
func SignUsingRawScalar(sBz []byte, message []byte) []byte {
	var err error
	if len(sBz) != 32 {
		panic(fmt.Sprintf("expected the scalar to be 32 bytes but got %d bytes", len(sBz)))
	}
	s, err := edwards25519.NewScalar().SetCanonicalBytes(sBz)
	if err != nil {
		panic(err)
	}

	rBz := make([]byte, 64)
	_, err = rand.Read(rBz)
	if err != nil {
		panic(err)
	}
	r, err := edwards25519.NewScalar().SetUniformBytes(rBz)
	if err != nil {
		panic(err)
	}
	A := (&edwards25519.Point{}).ScalarBaseMult(s)
	R := (&edwards25519.Point{}).ScalarBaseMult(r)

	Rbz := R.Bytes()
	Abz := A.Bytes()

	hasher := sha512.New()
	// k = SHA-512(R.BytesEd25519() || A.BytesEd25519() || M)
	hasher.Write(Rbz)
	hasher.Write(Abz)
	hasher.Write(message)
	kBz := hasher.Sum([]byte{})
	k, err := edwards25519.NewScalar().SetUniformBytes(kBz)
	if err != nil {
		panic(err)
	}
	// k * s + r
	S := edwards25519.NewScalar().MultiplyAdd(k, s, r)

	return append(R.Bytes(), S.Bytes()...)
}
