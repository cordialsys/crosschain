package signer_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"testing"

	"filippo.io/edwards25519"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/stretchr/testify/require"
)

func ScalarFromSeed(seed []byte) *edwards25519.Scalar {
	h := sha512.Sum512(seed[:32])
	s, err := edwards25519.NewScalar().SetBytesWithClamping(h[:32])
	if err != nil {
		panic(err)
	}
	return s
}

func TestEd255Raw(t *testing.T) {
	require := require.New(t)
	for i := 0; i < 16; i++ {
		seedBz := make([]byte, 32)
		_, err := rand.Read(seedBz)
		require.NoError(err)

		message := []byte("moar BTC")

		priv := ed25519.NewKeyFromSeed(seedBz)

		s := ScalarFromSeed(seedBz)

		sig := signer.SignWithScalar(s.Bytes(), message)

		// verify signature is valid using native lib
		valid := ed25519.Verify(priv.Public().(ed25519.PublicKey), message, sig)
		require.True(valid, "valid signature")
	}
}
