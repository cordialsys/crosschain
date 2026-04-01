package crypto

import (
	"filippo.io/edwards25519"
)

// CLSAGVerify verifies a CLSAG ring signature.
// Returns true if the signature is valid.
func CLSAGVerify(
	message []byte,
	ring []*edwards25519.Point,
	cNonzero []*edwards25519.Point,
	cOffset *edwards25519.Point,
	sig *CLSAGSignature,
) bool {
	n := len(ring)
	if n == 0 || n != len(sig.S) || n != len(cNonzero) {
		return false
	}

	I := sig.I
	// D_8 = 8 * sig.D
	eight := make([]byte, 32)
	eight[0] = 8
	eightScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(eight)
	D8 := edwards25519.NewIdentityPoint().ScalarMult(eightScalar, sig.D)

	// Adjusted commitments: C[i] = C_nonzero[i] - C_offset
	C := make([]*edwards25519.Point, n)
	negCOffset := edwards25519.NewIdentityPoint().Negate(cOffset)
	for i := 0; i < n; i++ {
		C[i] = edwards25519.NewIdentityPoint().Add(cNonzero[i], negCOffset)
	}

	// Aggregation hashes
	muPData := make([]byte, 0, 32*(2*n+4))
	muCData := make([]byte, 0, 32*(2*n+4))
	tag0 := make([]byte, 32)
	copy(tag0, "CLSAG_agg_0")
	tag1 := make([]byte, 32)
	copy(tag1, "CLSAG_agg_1")
	muPData = append(muPData, tag0...)
	muCData = append(muCData, tag1...)
	for i := 0; i < n; i++ {
		muPData = append(muPData, ring[i].Bytes()...)
		muCData = append(muCData, ring[i].Bytes()...)
	}
	for i := 0; i < n; i++ {
		muPData = append(muPData, cNonzero[i].Bytes()...)
		muCData = append(muCData, cNonzero[i].Bytes()...)
	}
	muPData = append(muPData, I.Bytes()...)
	muPData = append(muPData, sig.D.Bytes()...)
	muPData = append(muPData, cOffset.Bytes()...)
	muCData = append(muCData, I.Bytes()...)
	muCData = append(muCData, sig.D.Bytes()...)
	muCData = append(muCData, cOffset.Bytes()...)

	muP := hashToScalar(muPData)
	muC := hashToScalar(muCData)

	// Round hash prefix
	roundPrefix := make([]byte, 0, 32*(2*n+3))
	roundTag := make([]byte, 32)
	copy(roundTag, "CLSAG_round")
	roundPrefix = append(roundPrefix, roundTag...)
	for i := 0; i < n; i++ {
		roundPrefix = append(roundPrefix, ring[i].Bytes()...)
	}
	for i := 0; i < n; i++ {
		roundPrefix = append(roundPrefix, cNonzero[i].Bytes()...)
	}
	roundPrefix = append(roundPrefix, cOffset.Bytes()...)
	roundPrefix = append(roundPrefix, message...)

	// Start from c1, walk the ring
	c := scalarCopy(sig.C1)

	for i := 0; i < n; i++ {
		cP := scalarMul(muP, c)
		cC := scalarMul(muC, c)

		// L = s[i]*G + c_p*P[i] + c_c*C[i]
		siG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sig.S[i])
		cpPi := edwards25519.NewIdentityPoint().ScalarMult(cP, ring[i])
		ccCi := edwards25519.NewIdentityPoint().ScalarMult(cC, C[i])
		L := edwards25519.NewIdentityPoint().Add(siG, cpPi)
		L = edwards25519.NewIdentityPoint().Add(L, ccCi)

		// R = s[i]*H_p(P[i]) + c_p*I + c_c*D8
		hpPiBytes := HashToEC(ring[i].Bytes())
		hpPi, _ := edwards25519.NewIdentityPoint().SetBytes(hpPiBytes)
		siHp := edwards25519.NewIdentityPoint().ScalarMult(sig.S[i], hpPi)
		cpI := edwards25519.NewIdentityPoint().ScalarMult(cP, I)
		ccD8 := edwards25519.NewIdentityPoint().ScalarMult(cC, D8)
		R := edwards25519.NewIdentityPoint().Add(siHp, cpI)
		R = edwards25519.NewIdentityPoint().Add(R, ccD8)

		// Next challenge
		cData := make([]byte, 0, len(roundPrefix)+64)
		cData = append(cData, roundPrefix...)
		cData = append(cData, L.Bytes()...)
		cData = append(cData, R.Bytes()...)
		c = hashToScalar(cData)
	}

	// Check: c should equal sig.C1
	diff := scalarSub(c, sig.C1)
	return diff.Equal(edwards25519.NewScalar()) == 1
}
