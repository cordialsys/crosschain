package crypto

import (
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// CLSAGSignature represents a CLSAG ring signature.
type CLSAGSignature struct {
	S  []*edwards25519.Scalar // response scalars (one per ring member)
	C1 *edwards25519.Scalar   // initial challenge
	I  *edwards25519.Point    // key image: p * H_p(P[l])
	D  *edwards25519.Point    // commitment key image: (1/8) * z * H_p(P[l])
}

// CLSAGContext holds parameters for CLSAG signing.
type CLSAGContext struct {
	Message     []byte                // tx prefix hash (32 bytes)
	Ring        []*edwards25519.Point // P[0..n-1]: one-time public keys in the ring
	CNonzero    []*edwards25519.Point // C_nonzero[0..n-1]: original commitments from chain
	COffset     *edwards25519.Point   // pseudo-output commitment for this input
	SecretIndex int                   // position of real output in ring
	SecretKey   *edwards25519.Scalar  // p: one-time private key
	ZKey        *edwards25519.Scalar  // z = input_mask - pseudo_out_mask
	Rand        io.Reader             // optional deterministic RNG
}

// invEight is the scalar 1/8 mod L, used for scaling the commitment image D.
var invEight *edwards25519.Scalar

func init() {
	eight := make([]byte, 32)
	eight[0] = 8
	eightScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(eight)
	invEight = edwards25519.NewScalar().Invert(eightScalar)
}

// ComputeKeyImage computes I = x * H_p(P) for a given private key and public key.
// Uses pure Go hash_to_ec.
func ComputeKeyImage(privateKey *edwards25519.Scalar, publicKey *edwards25519.Point) *edwards25519.Point {
	hpBytes := HashToECPureGo(publicKey.Bytes())
	hp, _ := edwards25519.NewIdentityPoint().SetBytes(hpBytes)
	return edwards25519.NewIdentityPoint().ScalarMult(privateKey, hp)
}

// CLSAGSign produces a CLSAG ring signature following Monero's exact algorithm.
// Reference: monero/src/ringct/rctSigs.cpp CLSAG_Gen
func CLSAGSign(ctx *CLSAGContext) (*CLSAGSignature, error) {
	n := len(ctx.Ring)
	if n == 0 {
		return nil, fmt.Errorf("empty ring")
	}
	l := ctx.SecretIndex
	p := ctx.SecretKey
	z := ctx.ZKey

	// Compute adjusted commitments: C[i] = C_nonzero[i] - C_offset
	C := make([]*edwards25519.Point, n)
	negCOffset := edwards25519.NewIdentityPoint().Negate(ctx.COffset)
	for i := 0; i < n; i++ {
		C[i] = edwards25519.NewIdentityPoint().Add(ctx.CNonzero[i], negCOffset)
	}

	// Compute H_p(P[l]) - hash to point of signer's public key
	hpBytes := HashToEC(ctx.Ring[l].Bytes())
	Hp, _ := edwards25519.NewIdentityPoint().SetBytes(hpBytes)

	// Key image: I = p * H_p(P[l])
	I := edwards25519.NewIdentityPoint().ScalarMult(p, Hp)

	// Commitment image: D_full = z * H_p(P[l]), then D = (1/8) * D_full
	Dfull := edwards25519.NewIdentityPoint().ScalarMult(z, Hp)
	D := edwards25519.NewIdentityPoint().ScalarMult(invEight, Dfull)

	// Random nonce
	a := randomScalarFrom(ctx.Rand)

	// aG = a * G
	aG := edwards25519.NewGeneratorPoint().ScalarBaseMult(a)
	// aH = a * H_p(P[l])
	aH := edwards25519.NewIdentityPoint().ScalarMult(a, Hp)

	// --- Compute aggregation coefficients mu_P, mu_C ---
	// Uses sig.D (= D/8) in the hash, matching both prove and verify code.
	muPData := make([]byte, 0, 32*(2*n+4))
	muCData := make([]byte, 0, 32*(2*n+4))

	tag0 := make([]byte, 32)
	copy(tag0, "CLSAG_agg_0")
	tag1 := make([]byte, 32)
	copy(tag1, "CLSAG_agg_1")

	muPData = append(muPData, tag0...)
	muCData = append(muCData, tag1...)

	for i := 0; i < n; i++ {
		muPData = append(muPData, ctx.Ring[i].Bytes()...)
		muCData = append(muCData, ctx.Ring[i].Bytes()...)
	}
	for i := 0; i < n; i++ {
		muPData = append(muPData, ctx.CNonzero[i].Bytes()...)
		muCData = append(muCData, ctx.CNonzero[i].Bytes()...)
	}
	// I, sig.D (= D/8), C_offset
	muPData = append(muPData, I.Bytes()...)
	muPData = append(muPData, D.Bytes()...) // D here is already D/8 (sig.D)
	muPData = append(muPData, ctx.COffset.Bytes()...)
	muCData = append(muCData, I.Bytes()...)
	muCData = append(muCData, D.Bytes()...) // D here is already D/8 (sig.D)
	muCData = append(muCData, ctx.COffset.Bytes()...)

	muP := hashToScalar(muPData)
	muC := hashToScalar(muCData)

	// --- Build round hash template ---
	// "CLSAG_round" || P[0..n-1] || C_nonzero[0..n-1] || C_offset || message || L || R
	roundPrefix := make([]byte, 0, 32*(2*n+3))
	roundTag := make([]byte, 32)
	copy(roundTag, "CLSAG_round")
	roundPrefix = append(roundPrefix, roundTag...)
	for i := 0; i < n; i++ {
		roundPrefix = append(roundPrefix, ctx.Ring[i].Bytes()...)
	}
	for i := 0; i < n; i++ {
		roundPrefix = append(roundPrefix, ctx.CNonzero[i].Bytes()...)
	}
	roundPrefix = append(roundPrefix, ctx.COffset.Bytes()...)
	roundPrefix = append(roundPrefix, ctx.Message...)

	// Initial challenge: hash with aG and aH
	c0Data := make([]byte, 0, len(roundPrefix)+64)
	c0Data = append(c0Data, roundPrefix...)
	c0Data = append(c0Data, aG.Bytes()...)
	c0Data = append(c0Data, aH.Bytes()...)
	c := hashToScalar(c0Data)

	// Initialize s values
	s := make([]*edwards25519.Scalar, n)

	// Store c1 when we wrap around to index 0
	var c1 *edwards25519.Scalar

	i := (l + 1) % n
	if i == 0 {
		c1 = scalarCopy(c)
	}

	// --- Ring traversal ---
	for i != l {
		s[i] = randomScalarFrom(ctx.Rand)

		// c_p = mu_P * c, c_c = mu_C * c
		cP := scalarMul(muP, c)
		cC := scalarMul(muC, c)

		// L = s[i]*G + c_p*P[i] + c_c*C[i]
		siG := edwards25519.NewGeneratorPoint().ScalarBaseMult(s[i])
		cpPi := edwards25519.NewIdentityPoint().ScalarMult(cP, ctx.Ring[i])
		ccCi := edwards25519.NewIdentityPoint().ScalarMult(cC, C[i])
		L := edwards25519.NewIdentityPoint().Add(siG, cpPi)
		L = edwards25519.NewIdentityPoint().Add(L, ccCi)

		// R = s[i]*H_p(P[i]) + c_p*I + c_c*Dfull
		// Prove code line 268: D_precomp from D (full, NOT D/8)
		// Verify code line 897-902: D_precomp from 8*sig.D = D (full)
		// Both use the FULL D for ring computation.
		hpPiBytes := HashToEC(ctx.Ring[i].Bytes())
		hpPi, _ := edwards25519.NewIdentityPoint().SetBytes(hpPiBytes)
		siHp := edwards25519.NewIdentityPoint().ScalarMult(s[i], hpPi)
		cpI := edwards25519.NewIdentityPoint().ScalarMult(cP, I)
		ccDfull := edwards25519.NewIdentityPoint().ScalarMult(cC, Dfull)
		R := edwards25519.NewIdentityPoint().Add(siHp, cpI)
		R = edwards25519.NewIdentityPoint().Add(R, ccDfull)

		// Next challenge
		cData := make([]byte, 0, len(roundPrefix)+64)
		cData = append(cData, roundPrefix...)
		cData = append(cData, L.Bytes()...)
		cData = append(cData, R.Bytes()...)
		c = hashToScalar(cData)

		i = (i + 1) % n
		if i == 0 {
			c1 = scalarCopy(c)
		}
	}

	// --- Close the ring ---
	// s[l] = a - c * (mu_P * p + mu_C * z)
	muPp := scalarMul(muP, p)
	muCz := scalarMul(muC, z)
	secret := scalarAdd(muPp, muCz)
	s[l] = scalarSub(a, scalarMul(c, secret))

	if c1 == nil {
		// This happens when l == n-1 and we never wrapped to 0
		// c1 should be the challenge computed after s[n-1]
		// which is the initial c we started with from (l+1) % n = 0
		// Actually if l = n-1, then i starts at 0, and c1 is set immediately.
		// So c1 should always be set. Just in case:
		c1 = scalarCopy(c)
	}

	return &CLSAGSignature{
		S:  s,
		C1: c1,
		I:  I,
		D:  D,
	}, nil
}

// SerializeCLSAGWithKeyImage serializes a CLSAG signature + key image.
// Format: key_image(32) || s[0](32) || ... || s[n-1](32) || c1(32) || D(32)
func SerializeCLSAGWithKeyImage(sig *CLSAGSignature, keyImage *edwards25519.Point) []byte {
	var out []byte
	out = append(out, keyImage.Bytes()...)
	for _, s := range sig.S {
		out = append(out, s.Bytes()...)
	}
	out = append(out, sig.C1.Bytes()...)
	out = append(out, sig.D.Bytes()...)
	return out
}

// DeserializeCLSAG parses a CLSAG signature + key image from bytes.
// Returns (signature, keyImage, error).
func DeserializeCLSAG(data []byte, ringSize int) (*CLSAGSignature, []byte, error) {
	expected := 32 + ringSize*32 + 32 + 32 // keyImage + s[] + c1 + D
	if len(data) != expected {
		return nil, nil, fmt.Errorf("expected %d bytes, got %d", expected, len(data))
	}

	pos := 0
	keyImage := data[pos : pos+32]
	pos += 32

	s := make([]*edwards25519.Scalar, ringSize)
	for i := 0; i < ringSize; i++ {
		s[i], _ = edwards25519.NewScalar().SetCanonicalBytes(data[pos : pos+32])
		pos += 32
	}

	c1, _ := edwards25519.NewScalar().SetCanonicalBytes(data[pos : pos+32])
	pos += 32

	D, _ := edwards25519.NewIdentityPoint().SetBytes(data[pos : pos+32])

	// Reconstruct I from key image bytes
	I, _ := edwards25519.NewIdentityPoint().SetBytes(keyImage)

	return &CLSAGSignature{S: s, C1: c1, I: I, D: D}, keyImage, nil
}

// SerializeCLSAG serializes a CLSAG signature to bytes.
// Format: s[0] || s[1] || ... || s[n-1] || c1 || D
func (sig *CLSAGSignature) Serialize() []byte {
	var out []byte
	for _, s := range sig.S {
		out = append(out, s.Bytes()...)
	}
	out = append(out, sig.C1.Bytes()...)
	out = append(out, sig.D.Bytes()...)
	return out
}

func hashToScalar(data []byte) *edwards25519.Scalar {
	hash := Keccak256(data)
	reduced := ScReduce32(hash)
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(reduced)
	return s
}
