package crypto

import (
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestCLSAGSignAndVerify(t *testing.T) {
	// Create a simple ring of size 4 with random keys
	n := 4
	secretIndex := 2

	// Generate random ring members
	ring := make([]*edwards25519.Point, n)
	commitments := make([]*edwards25519.Point, n)

	for i := 0; i < n; i++ {
		sk := randomScalar()
		ring[i] = edwards25519.NewGeneratorPoint().ScalarBaseMult(sk)
		mask := randomScalar()
		commitments[i], _ = PedersenCommit(uint64(1000*(i+1)), mask.Bytes())
	}

	// Real signer's keys
	privKey := randomScalar()
	ring[secretIndex] = edwards25519.NewGeneratorPoint().ScalarBaseMult(privKey)

	// Real signer's commitment
	amount := uint64(5000)
	inputMask := randomScalar()
	commitments[secretIndex], _ = PedersenCommit(amount, inputMask.Bytes())

	// Pseudo-output commitment (must balance)
	pseudoMask := randomScalar()
	cOffset, _ := PedersenCommit(amount, pseudoMask.Bytes())

	// z = input_mask - pseudo_mask
	z := scalarSub(inputMask, pseudoMask)

	message := Keccak256([]byte("test message for CLSAG"))

	ctx := &CLSAGContext{
		Message:     message,
		Ring:        ring,
		CNonzero:    commitments,
		COffset:     cOffset,
		SecretIndex: secretIndex,
		SecretKey:   privKey,
		ZKey:        z,
	}

	sig, err := CLSAGSign(ctx)
	require.NoError(t, err)
	require.NotNil(t, sig)
	require.Len(t, sig.S, n)

	// Verify
	valid := CLSAGVerify(message, ring, commitments, cOffset, sig)
	require.True(t, valid, "CLSAG signature should verify")

	// Verify with wrong message fails
	wrongMsg := Keccak256([]byte("wrong message"))
	valid2 := CLSAGVerify(wrongMsg, ring, commitments, cOffset, sig)
	require.False(t, valid2, "CLSAG with wrong message should not verify")
}

func TestCLSAGRingSize16(t *testing.T) {
	// Test with realistic ring size of 16
	n := 16
	secretIndex := 7

	ring := make([]*edwards25519.Point, n)
	commitments := make([]*edwards25519.Point, n)
	for i := 0; i < n; i++ {
		sk := randomScalar()
		ring[i] = edwards25519.NewGeneratorPoint().ScalarBaseMult(sk)
		mask := randomScalar()
		commitments[i], _ = PedersenCommit(uint64(1000*(i+1)), mask.Bytes())
	}

	privKey := randomScalar()
	ring[secretIndex] = edwards25519.NewGeneratorPoint().ScalarBaseMult(privKey)

	amount := uint64(43910000000) // our deposit amount
	inputMask := randomScalar()
	commitments[secretIndex], _ = PedersenCommit(amount, inputMask.Bytes())

	pseudoMask := randomScalar()
	cOffset, _ := PedersenCommit(amount, pseudoMask.Bytes())
	z := scalarSub(inputMask, pseudoMask)

	message := Keccak256([]byte("ring size 16 test"))

	ctx := &CLSAGContext{
		Message:     message,
		Ring:        ring,
		CNonzero:    commitments,
		COffset:     cOffset,
		SecretIndex: secretIndex,
		SecretKey:   privKey,
		ZKey:        z,
	}

	sig, err := CLSAGSign(ctx)
	require.NoError(t, err)

	valid := CLSAGVerify(message, ring, commitments, cOffset, sig)
	require.True(t, valid, "CLSAG with ring size 16 should verify")
}
