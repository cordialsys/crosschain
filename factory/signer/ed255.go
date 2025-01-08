package signer

import (
	"crypto/rand"
	"crypto/sha512"
	"fmt"

	"filippo.io/edwards25519"
)

// With regular RFC 8032 Ed25519 signing, a private key is in 64 byte {seed, public-key} format.
// During signing, the seed is hashed to derive the actual scalar.  This is
// incompatible with MPC algorithms, so we provide this alternative signing implementation, which is essentially FROST-Ed25519 signing for the case t = n = 1.
// This will perform the signing using the scalar itself -- what would be part of the hashed output of "seed".
//
// For more background, see:
// - https://github.com/taurushq-io/frost-ed25519/blob/master/README.md#signatures
// - https://blog.web3auth.io/introducing-ed25519-in-web3auths-mpc-secure-signing-for-dapps-and-wallets/
func SignWithScalar(sBz []byte, message []byte) []byte {
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
