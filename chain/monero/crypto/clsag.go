package crypto

import (
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// CLSAGSignature represents a CLSAG (Concise Linkable Spontaneous Anonymous Group) ring signature.
type CLSAGSignature struct {
	// S are the response scalars (one per ring member)
	S []*edwards25519.Scalar
	// C1 is the initial challenge scalar
	C1 *edwards25519.Scalar
	// D is the auxiliary key image component (for commitment signing)
	D *edwards25519.Point
}

// CLSAGContext holds the parameters needed for CLSAG signing.
type CLSAGContext struct {
	// Message is the data being signed (tx prefix hash)
	Message []byte
	// Ring is the set of public keys (one-time output keys) in the ring
	Ring []*edwards25519.Point
	// Commitments are the Pedersen commitments for each ring member
	Commitments []*edwards25519.Point
	// PseudoOut is the pseudo-output commitment for this input
	PseudoOut *edwards25519.Point
	// KeyImage is I = x * H_p(P) where x is the private key and P is the real output key
	KeyImage *edwards25519.Point
	// SecretIndex is the position of the real output in the ring
	SecretIndex int
	// SecretKey is the one-time private key for the real output
	SecretKey *edwards25519.Scalar
	// CommitmentMask is the difference between real commitment mask and pseudo-out mask
	// z = input_mask - pseudo_out_mask
	CommitmentMask *edwards25519.Scalar
	// Rand is an optional deterministic random reader
	Rand io.Reader
}

// ComputeKeyImage computes I = x * H_p(P) where:
//   - x is the private spend key for this output
//   - P is the one-time public key of the output
//   - H_p is hash-to-point
func ComputeKeyImage(privateKey *edwards25519.Scalar, publicKey *edwards25519.Point) *edwards25519.Point {
	hp := hashToPoint(publicKey.Bytes())
	return edwards25519.NewIdentityPoint().ScalarMult(privateKey, hp)
}

// CLSAGSign produces a CLSAG ring signature.
//
// The algorithm:
//  1. Compute auxiliary values: mu_P = H("CLSAG_agg_0" || ring || I || ...), mu_C = H("CLSAG_agg_1" || ...)
//  2. Generate random nonce alpha
//  3. Compute initial commitments: alpha*G and alpha*H_p(P[l])
//  4. Walk the ring computing challenges: c[i+1] = H(msg || ... || s[i]*G + c[i]*mu_P*P[i] || ...)
//  5. Close the ring: s[l] = alpha - c[l] * (mu_P*x + mu_C*z)
func CLSAGSign(ctx *CLSAGContext) (*CLSAGSignature, error) {
	ringSize := len(ctx.Ring)
	if ringSize == 0 {
		return nil, fmt.Errorf("empty ring")
	}
	if ctx.SecretIndex < 0 || ctx.SecretIndex >= ringSize {
		return nil, fmt.Errorf("secret index %d out of range [0, %d)", ctx.SecretIndex, ringSize)
	}
	if len(ctx.Commitments) != ringSize {
		return nil, fmt.Errorf("commitments count %d != ring size %d", len(ctx.Commitments), ringSize)
	}

	l := ctx.SecretIndex
	x := ctx.SecretKey
	z := ctx.CommitmentMask
	P := ctx.Ring
	C := ctx.Commitments
	I := ctx.KeyImage
	Cout := ctx.PseudoOut

	// Compute commitment differences: C[i] - Cout
	Cdiff := make([]*edwards25519.Point, ringSize)
	for i := 0; i < ringSize; i++ {
		negCout := edwards25519.NewIdentityPoint().Negate(Cout)
		Cdiff[i] = edwards25519.NewIdentityPoint().Add(C[i], negCout)
	}

	// Compute D = z * H_p(P[l])  (auxiliary key image for commitment)
	hpPl := hashToPoint(P[l].Bytes())
	D := edwards25519.NewIdentityPoint().ScalarMult(z, hpPl)

	// Compute aggregation coefficients mu_P and mu_C
	// mu_P = H_s("CLSAG_agg_0" || ring_data || I || D || Cout)
	// mu_C = H_s("CLSAG_agg_1" || ring_data || I || D || Cout)
	aggData0 := []byte("CLSAG_agg_0")
	aggData1 := []byte("CLSAG_agg_1")
	ringData := buildRingData(P, C)
	aggData0 = append(aggData0, ringData...)
	aggData0 = append(aggData0, I.Bytes()...)
	aggData0 = append(aggData0, D.Bytes()...)
	aggData0 = append(aggData0, Cout.Bytes()...)
	aggData1 = append(aggData1, ringData...)
	aggData1 = append(aggData1, I.Bytes()...)
	aggData1 = append(aggData1, D.Bytes()...)
	aggData1 = append(aggData1, Cout.Bytes()...)

	muP := hashToScalar(aggData0)
	muC := hashToScalar(aggData1)

	// Generate random nonce
	alpha := randomScalarFrom(ctx.Rand)

	// Compute initial round values at position l:
	// aG  = alpha * G
	// aHp = alpha * H_p(P[l])
	aG := edwards25519.NewGeneratorPoint().ScalarBaseMult(alpha)
	aHp := edwards25519.NewIdentityPoint().ScalarMult(alpha, hpPl)

	// Initialize response scalars with random values for all positions except l
	s := make([]*edwards25519.Scalar, ringSize)
	for i := 0; i < ringSize; i++ {
		if i != l {
			s[i] = randomScalarFrom(ctx.Rand)
		}
	}

	// Compute c[l+1] from the initial commitment
	c := make([]*edwards25519.Scalar, ringSize)
	cData := buildChallengeData(ctx.Message, aG, aHp, P, Cdiff, I, D, l)
	c[(l+1)%ringSize] = hashToScalar(cData)

	// Walk the ring from l+1 to l-1
	for j := 1; j < ringSize; j++ {
		i := (l + j) % ringSize

		// W1 = s[i]*G + c[i] * (mu_P*P[i] + mu_C*Cdiff[i])
		siG := edwards25519.NewGeneratorPoint().ScalarBaseMult(s[i])
		muPPi := edwards25519.NewIdentityPoint().ScalarMult(muP, P[i])
		muCCi := edwards25519.NewIdentityPoint().ScalarMult(muC, Cdiff[i])
		combined := edwards25519.NewIdentityPoint().Add(muPPi, muCCi)
		ciCombined := edwards25519.NewIdentityPoint().ScalarMult(c[i], combined)
		W1 := edwards25519.NewIdentityPoint().Add(siG, ciCombined)

		// W2 = s[i]*H_p(P[i]) + c[i] * (mu_P*I + mu_C*D)
		hpPi := hashToPoint(P[i].Bytes())
		siHp := edwards25519.NewIdentityPoint().ScalarMult(s[i], hpPi)
		muPI := edwards25519.NewIdentityPoint().ScalarMult(muP, I)
		muCD := edwards25519.NewIdentityPoint().ScalarMult(muC, D)
		imgCombined := edwards25519.NewIdentityPoint().Add(muPI, muCD)
		ciImg := edwards25519.NewIdentityPoint().ScalarMult(c[i], imgCombined)
		W2 := edwards25519.NewIdentityPoint().Add(siHp, ciImg)

		// c[i+1] = H_s(msg || W1 || W2)
		nextData := buildChallengeDataFromPoints(ctx.Message, W1, W2, P, Cdiff, I, D, i)
		c[(i+1)%ringSize] = hashToScalar(nextData)
	}

	// Close the ring: s[l] = alpha - c[l] * (mu_P * x + mu_C * z)
	muPx := scalarMul(muP, x)
	muCz := scalarMul(muC, z)
	secret := scalarAdd(muPx, muCz)
	s[l] = scalarSub(alpha, scalarMul(c[l], secret))

	return &CLSAGSignature{
		S:  s,
		C1: c[0],
		D:  D,
	}, nil
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

func buildRingData(P []*edwards25519.Point, C []*edwards25519.Point) []byte {
	var data []byte
	for _, p := range P {
		data = append(data, p.Bytes()...)
	}
	for _, c := range C {
		data = append(data, c.Bytes()...)
	}
	return data
}

func buildChallengeData(msg []byte, aG, aHp *edwards25519.Point,
	P []*edwards25519.Point, Cdiff []*edwards25519.Point,
	I, D *edwards25519.Point, round int) []byte {
	var data []byte
	data = append(data, []byte("CLSAG_round")...)
	data = append(data, msg...)
	data = append(data, aG.Bytes()...)
	data = append(data, aHp.Bytes()...)
	return data
}

func buildChallengeDataFromPoints(msg []byte, W1, W2 *edwards25519.Point,
	P []*edwards25519.Point, Cdiff []*edwards25519.Point,
	I, D *edwards25519.Point, round int) []byte {
	var data []byte
	data = append(data, []byte("CLSAG_round")...)
	data = append(data, msg...)
	data = append(data, W1.Bytes()...)
	data = append(data, W2.Bytes()...)
	return data
}

func hashToScalar(data []byte) *edwards25519.Scalar {
	hash := Keccak256(data)
	reduced := ScalarReduce(hash)
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(reduced)
	return s
}
