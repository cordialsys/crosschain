package crypto

// Pure Go Bulletproofs+ implementation.
// Ported from Monero's src/ringct/bulletproofs_plus.cc

import (
	"crypto/rand"
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// BP+ constants maxN, maxM, maxMN are defined in generators.go.

var (
	scOne      *edwards25519.Scalar
	scTwo      *edwards25519.Scalar
	scMinusOne *edwards25519.Scalar
	scInvEight *edwards25519.Scalar
	scMinusInvEight *edwards25519.Scalar

	initialTranscriptBytes [32]byte
)

func init() {
	oneBytes := make([]byte, 32)
	oneBytes[0] = 1
	scOne, _ = edwards25519.NewScalar().SetCanonicalBytes(oneBytes)

	twoBytes := make([]byte, 32)
	twoBytes[0] = 2
	scTwo, _ = edwards25519.NewScalar().SetCanonicalBytes(twoBytes)

	scMinusOne = edwards25519.NewScalar().Negate(scOne)

	// 8^(-1) mod L
	eightBytes := make([]byte, 32)
	eightBytes[0] = 8
	scEight, _ := edwards25519.NewScalar().SetCanonicalBytes(eightBytes)
	scInvEight = edwards25519.NewScalar().Invert(scEight)

	scMinusInvEight = edwards25519.NewScalar().Negate(scInvEight)

	// initial_transcript = hash_to_p3(cn_fast_hash("bulletproof_plus_transcript"))
	// hash_to_p3(k) = ge_fromfe(cn_fast_hash(k)) * 8 -- note the DOUBLE hash!
	h := Keccak256([]byte("bulletproof_plus_transcript")) // first hash
	h2 := Keccak256(h)                                     // hash_to_p3 hashes again internally
	point := geFromfeFrombytesVartime(h2)
	p2 := edwards25519.NewIdentityPoint().Add(point, point)
	p4 := edwards25519.NewIdentityPoint().Add(p2, p2)
	p8 := edwards25519.NewIdentityPoint().Add(p4, p4)
	copy(initialTranscriptBytes[:], p8.Bytes())
}

// BPPlusProvePureGo generates a Bulletproofs+ range proof in pure Go.
func BPPlusProvePureGo(amounts []uint64, masks [][]byte, randReader ...io.Reader) ([]byte, error) {
	m := len(amounts)
	if m == 0 || m > maxM || len(masks) != m {
		return nil, fmt.Errorf("invalid BP+ inputs: %d amounts, %d masks", m, len(masks))
	}

	var rng io.Reader
	if len(randReader) > 0 && randReader[0] != nil {
		rng = randReader[0]
	}

	// M = next power of 2 >= m
	M := 1
	logM := 0
	for M < m {
		M <<= 1
		logM++
	}
	logN := 6 // log2(64)
	N := 1 << logN
	logMN := logM + logN
	MN := M * N

	// Convert amounts to scalars
	sv := make([]*edwards25519.Scalar, m)
	for i, a := range amounts {
		sv[i], _ = edwards25519.NewScalar().SetCanonicalBytes(ScalarFromUint64(a))
	}
	gammas := make([]*edwards25519.Scalar, m)
	for i, mask := range masks {
		gammas[i], _ = edwards25519.NewScalar().SetCanonicalBytes(mask)
	}

	// Compute V[i] = (gamma/8)*G + (v/8)*H
	V := make([]*edwards25519.Point, m)
	for i := range sv {
		gamma8 := scMul(gammas[i], scInvEight)
		sv8 := scMul(sv[i], scInvEight)
		V[i] = addKeys2(gamma8, sv8, H)
	}

	// Decompose values into bits
	aL := make([]*edwards25519.Scalar, MN)
	aR := make([]*edwards25519.Scalar, MN)
	aL8 := make([]*edwards25519.Scalar, MN)
	aR8 := make([]*edwards25519.Scalar, MN)

	for j := 0; j < M; j++ {
		for i := N - 1; i >= 0; i-- {
			idx := j*N + i
			if j < m && (amounts[j]>>(i%64))&1 == 1 {
				aL[idx] = scalarCopy(scOne)
				aL8[idx] = scalarCopy(scInvEight)
				aR[idx] = edwards25519.NewScalar()
				aR8[idx] = edwards25519.NewScalar()
			} else {
				aL[idx] = edwards25519.NewScalar()
				aL8[idx] = edwards25519.NewScalar()
				aR[idx] = scalarCopy(scMinusOne)
				aR8[idx] = scalarCopy(scMinusInvEight)
			}
		}
	}

	// Fiat-Shamir transcript (raw 32-byte keys, not scalars)
	ensureHtpInit()
	transcript := initialTranscriptBytes
	transcript, _ = transcriptUpdateKey(transcript, keyFromScalar(hashKeyV(V)))

	// A = alpha*G/8 + sum(aL8[i]*Gi[i] + aR8[i]*Hi[i])
	alpha := skGen(rng)
	preA := vectorExponent(aL8, aR8, MN)
	alphaInv8 := scMul(alpha, scInvEight)
	A := ptAdd(preA, ptScalarBaseMult(alphaInv8))

	// Challenges y, z
	var y *edwards25519.Scalar
	transcript, y = transcriptUpdateKey(transcript, keyFromPoint(A))
	if y.Equal(edwards25519.NewScalar()) == 1 {
		return nil, fmt.Errorf("y is 0")
	}
	// z = hash_to_scalar(y)
	zReduced := ScReduce32(Keccak256(y.Bytes()))
	var z *edwards25519.Scalar
	z, _ = edwards25519.NewScalar().SetCanonicalBytes(zReduced)
	copy(transcript[:], zReduced) // transcript = z
	if z.Equal(edwards25519.NewScalar()) == 1 {
		return nil, fmt.Errorf("z is 0")
	}
	zSq := scMul(z, z)

	// d[j*N+i] = z^(2*(j+1)) * 2^i
	d := make([]*edwards25519.Scalar, MN)
	d[0] = scalarCopy(zSq)
	for i := 1; i < N; i++ {
		d[i] = scMul(d[i-1], scTwo)
	}
	for j := 1; j < M; j++ {
		for i := 0; i < N; i++ {
			d[j*N+i] = scMul(d[(j-1)*N+i], zSq)
		}
	}

	// y powers: y^0 ... y^(MN+1)
	yPow := scalarPowers(y, MN+2)
	yInv := scalarInvert(y)
	yInvPow := scalarPowers(yInv, MN)

	// aL1 = aL - z, aR1 = aR + z + d_y
	aL1 := make([]*edwards25519.Scalar, MN)
	aR1 := make([]*edwards25519.Scalar, MN)
	for i := 0; i < MN; i++ {
		aL1[i] = scalarSub(aL[i], z)
		dy := scMul(d[i], yPow[MN-i])
		aR1[i] = scalarAdd(scalarAdd(aR[i], z), dy)
	}

	// alpha1 = alpha + sum(z^(2*(j+1)) * y^(MN+1) * gamma[j])
	alpha1 := scalarCopy(alpha)
	temp := scalarCopy(scOne)
	for j := 0; j < m; j++ {
		temp = scMul(temp, zSq)
		t2 := scMul(yPow[MN+1], temp)
		alpha1 = scalarAdd(alpha1, scMul(t2, gammas[j]))
	}

	// Inner product rounds - track folded generators
	nprime := MN
	aprime := aL1
	bprime := aR1
	Gprime := make([]*edwards25519.Point, MN)
	Hprime := make([]*edwards25519.Point, MN)
	for i := 0; i < MN; i++ {
		Gprime[i] = ptCopy(Gi[i])
		Hprime[i] = ptCopy(Hi[i])
	}

	Lpoints := make([]*edwards25519.Point, logMN)
	Rpoints := make([]*edwards25519.Point, logMN)
	round := 0

	for nprime > 1 {
		nprime /= 2

		cL := weightedInnerProduct(aprime[:nprime], bprime[nprime:2*nprime], y)
		aPrimeHigh := vectorScalar(aprime[nprime:2*nprime], yPow[nprime])
		cR := weightedInnerProduct(aPrimeHigh, bprime[:nprime], y)

		dL := skGen(rng)
		dR := skGen(rng)

		Lpoints[round] = computeLRWithGens(nprime, yInvPow[nprime],
			Gprime[nprime:], Hprime[:nprime], aprime[:nprime], bprime[nprime:2*nprime], cL, dL)
		Rpoints[round] = computeLRWithGens(nprime, yPow[nprime],
			Gprime[:nprime], Hprime[nprime:], aprime[nprime:2*nprime], bprime[:nprime], cR, dR)

		var challenge *edwards25519.Scalar
		transcript, challenge = transcriptUpdateKey2(transcript, keyFromPoint(Lpoints[round]), keyFromPoint(Rpoints[round]))
		if challenge.Equal(edwards25519.NewScalar()) == 1 {
			return nil, fmt.Errorf("challenge is 0")
		}
		challengeInv := scalarInvert(challenge)

		// Fold generators: Gprime[i] = challenge_inv * Gprime[i] + yinvpow[nprime]*challenge * Gprime[nprime+i]
		// Hprime[i] = challenge * Hprime[i] + challenge_inv * Hprime[nprime+i]
		yinvCh := scMul(yInvPow[nprime], challenge)
		for i := 0; i < nprime; i++ {
			gLo := edwards25519.NewIdentityPoint().ScalarMult(challengeInv, Gprime[i])
			gHi := edwards25519.NewIdentityPoint().ScalarMult(yinvCh, Gprime[nprime+i])
			Gprime[i] = ptAdd(gLo, gHi)

			hLo := edwards25519.NewIdentityPoint().ScalarMult(challenge, Hprime[i])
			hHi := edwards25519.NewIdentityPoint().ScalarMult(challengeInv, Hprime[nprime+i])
			Hprime[i] = ptAdd(hLo, hHi)
		}
		Gprime = Gprime[:nprime]
		Hprime = Hprime[:nprime]

		// Fold scalar vectors
		tempSc := scMul(challengeInv, yPow[nprime])
		aprime = vectorAdd(vectorScalar(aprime[:nprime], challenge), vectorScalar(aprime[nprime:2*nprime], tempSc))
		bprime = vectorAdd(vectorScalar(bprime[:nprime], challengeInv), vectorScalar(bprime[nprime:2*nprime], challenge))

		// Update alpha1
		chSq := scMul(challenge, challenge)
		chInvSq := scMul(challengeInv, challengeInv)
		alpha1 = scalarAdd(alpha1, scalarAdd(scMul(dL, chSq), scMul(dR, chInvSq)))

		round++
	}

	// Final round
	r := skGen(rng)
	s := skGen(rng)
	dFinal := skGen(rng)
	eta := skGen(rng)

	// A1 = r/8*Gprime[0] + s/8*Hprime[0] + d/8*G + (r*y*b' + s*y*a')/8 * H
	rInv8 := scMul(r, scInvEight)
	sInv8 := scMul(s, scInvEight)
	dInv8 := scMul(dFinal, scInvEight)
	ryb := scMul(scMul(r, y), bprime[0])
	sya := scMul(scMul(s, y), aprime[0])
	combInv8 := scMul(scalarAdd(ryb, sya), scInvEight)

	rG0 := edwards25519.NewIdentityPoint().ScalarMult(rInv8, Gprime[0])
	sH0 := edwards25519.NewIdentityPoint().ScalarMult(sInv8, Hprime[0])
	dG := ptScalarBaseMult(dInv8)
	cH := edwards25519.NewIdentityPoint().ScalarMult(combInv8, H)
	A1 := ptAdd(ptAdd(rG0, sH0), ptAdd(dG, cH))

	// B = eta/8*G + (r*y*s)/8*H
	rys := scMul(scMul(r, y), s)
	rysInv8 := scMul(rys, scInvEight)
	etaInv8 := scMul(eta, scInvEight)
	B := addKeys2(etaInv8, rysInv8, H)

	// Final challenge
	var e_challenge *edwards25519.Scalar
	transcript, e_challenge = transcriptUpdateKey2(transcript, keyFromPoint(A1), keyFromPoint(B))
	_ = transcript
	if e_challenge.Equal(edwards25519.NewScalar()) == 1 {
		return nil, fmt.Errorf("e is 0")
	}
	eSq := scMul(e_challenge, e_challenge)

	r1 := scalarAdd(r, scMul(aprime[0], e_challenge))
	s1 := scalarAdd(s, scMul(bprime[0], e_challenge))
	d1 := scalarAdd(eta, scalarAdd(scMul(dFinal, e_challenge), scMul(alpha1, eSq)))

	// Serialize the proof
	// Format: [4B nV] [nV*32 V] [32 A] [32 A1] [32 B] [32 r1] [32 s1] [32 d1] [4B nL] [nL*32 L] [4B nR] [nR*32 R]
	var out []byte
	writeU32 := func(v uint32) { out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	writeKey := func(b []byte) { out = append(out, b...) }

	writeU32(uint32(m))
	for _, v := range V {
		writeKey(v.Bytes())
	}
	writeKey(A.Bytes())
	writeKey(A1.Bytes())
	writeKey(B.Bytes())
	writeKey(r1.Bytes())
	writeKey(s1.Bytes())
	writeKey(d1.Bytes())
	writeU32(uint32(logMN))
	for i := 0; i < logMN; i++ {
		writeKey(Lpoints[i].Bytes())
	}
	writeU32(uint32(logMN))
	for i := 0; i < logMN; i++ {
		writeKey(Rpoints[i].Bytes())
	}

	return out, nil
}

// --- Helper functions ---

func skGen(rng io.Reader) *edwards25519.Scalar {
	entropy := make([]byte, 64)
	if rng != nil {
		rng.Read(entropy)
	} else {
		rand.Read(entropy)
	}
	s, _ := edwards25519.NewScalar().SetUniformBytes(entropy)
	return s
}

func scMul(a, b *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Multiply(a, b)
}

func addKeys2(aScalar, bScalar *edwards25519.Scalar, bPoint *edwards25519.Point) *edwards25519.Point {
	// aScalar*G + bScalar*bPoint
	aG := edwards25519.NewGeneratorPoint().ScalarBaseMult(aScalar)
	bP := edwards25519.NewIdentityPoint().ScalarMult(bScalar, bPoint)
	return edwards25519.NewIdentityPoint().Add(aG, bP)
}

func ptAdd(a, b *edwards25519.Point) *edwards25519.Point {
	return edwards25519.NewIdentityPoint().Add(a, b)
}

func ptScalarBaseMult(s *edwards25519.Scalar) *edwards25519.Point {
	return edwards25519.NewGeneratorPoint().ScalarBaseMult(s)
}

func ptToScalar(p *edwards25519.Point) *edwards25519.Scalar {
	h := Keccak256(p.Bytes())
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(ScReduce32(h))
	return s
}

func hashScalar(data []byte) *edwards25519.Scalar {
	h := Keccak256(data)
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(ScReduce32(h))
	return s
}

func hashKeyV(keys []*edwards25519.Point) *edwards25519.Scalar {
	var data []byte
	for _, k := range keys {
		data = append(data, k.Bytes()...)
	}
	return hashScalar(data)
}

// transcriptUpdateKey updates the transcript with one 32-byte key (raw bytes).
// Returns hash_to_scalar(transcript || update) as both raw bytes and scalar.
func transcriptUpdateKey(transcript [32]byte, update [32]byte) ([32]byte, *edwards25519.Scalar) {
	var data []byte
	data = append(data, transcript[:]...)
	data = append(data, update[:]...)
	h := Keccak256(data)
	reduced := ScReduce32(h)
	var result [32]byte
	copy(result[:], reduced)
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(reduced)
	return result, s
}

// transcriptUpdateKey2 updates the transcript with two 32-byte keys.
func transcriptUpdateKey2(transcript [32]byte, u0, u1 [32]byte) ([32]byte, *edwards25519.Scalar) {
	var data []byte
	data = append(data, transcript[:]...)
	data = append(data, u0[:]...)
	data = append(data, u1[:]...)
	h := Keccak256(data)
	reduced := ScReduce32(h)
	var result [32]byte
	copy(result[:], reduced)
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(reduced)
	return result, s
}

func keyFromPoint(p *edwards25519.Point) [32]byte {
	var k [32]byte
	copy(k[:], p.Bytes())
	return k
}

func keyFromScalar(s *edwards25519.Scalar) [32]byte {
	var k [32]byte
	copy(k[:], s.Bytes())
	return k
}

func vectorExponent(a, b []*edwards25519.Scalar, n int) *edwards25519.Point {
	result := edwards25519.NewIdentityPoint()
	for i := 0; i < n; i++ {
		aGi := edwards25519.NewIdentityPoint().ScalarMult(a[i], Gi[i])
		bHi := edwards25519.NewIdentityPoint().ScalarMult(b[i], Hi[i])
		result = edwards25519.NewIdentityPoint().Add(result, aGi)
		result = edwards25519.NewIdentityPoint().Add(result, bHi)
	}
	return result
}

func weightedInnerProduct(a, b []*edwards25519.Scalar, y *edwards25519.Scalar) *edwards25519.Scalar {
	result := edwards25519.NewScalar()
	yPow := scalarCopy(scOne)
	for i := range a {
		yPow = scMul(yPow, y)
		t := scMul(a[i], scMul(yPow, b[i]))
		result = scalarAdd(result, t)
	}
	return result
}

func computeLRWithGens(size int, yPow *edwards25519.Scalar,
	G []*edwards25519.Point, Hg []*edwards25519.Point,
	a []*edwards25519.Scalar, b []*edwards25519.Scalar,
	c, d *edwards25519.Scalar) *edwards25519.Point {
	// L or R = sum(a[i]*yPow/8*G[i] + b[i]/8*H[i]) + c/8*H + d/8*G_base
	result := edwards25519.NewIdentityPoint()
	for i := 0; i < size; i++ {
		aYi := scMul(scMul(a[i], yPow), scInvEight)
		bI := scMul(b[i], scInvEight)
		aGi := edwards25519.NewIdentityPoint().ScalarMult(aYi, G[i])
		bHi := edwards25519.NewIdentityPoint().ScalarMult(bI, Hg[i])
		result = ptAdd(result, aGi)
		result = ptAdd(result, bHi)
	}
	cH := edwards25519.NewIdentityPoint().ScalarMult(scMul(c, scInvEight), H)
	dG := ptScalarBaseMult(scMul(d, scInvEight))
	result = ptAdd(result, cH)
	result = ptAdd(result, dG)
	return result
}

func ptCopy(p *edwards25519.Point) *edwards25519.Point {
	return edwards25519.NewIdentityPoint().Add(p, edwards25519.NewIdentityPoint())
}

func vectorAdd(a, b []*edwards25519.Scalar) []*edwards25519.Scalar {
	r := make([]*edwards25519.Scalar, len(a))
	for i := range a {
		r[i] = scalarAdd(a[i], b[i])
	}
	return r
}

func vectorScalar(v []*edwards25519.Scalar, s *edwards25519.Scalar) []*edwards25519.Scalar {
	r := make([]*edwards25519.Scalar, len(v))
	for i := range v {
		r[i] = scMul(v[i], s)
	}
	return r
}
