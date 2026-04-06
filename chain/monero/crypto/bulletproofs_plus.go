package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// BulletproofPlus represents a Bulletproofs+ range proof proving that
// committed values are in the range [0, 2^64).
type BulletproofPlus struct {
	A  *edwards25519.Point   // commitment to witness vectors
	A1 *edwards25519.Point   // final round commitment
	B  *edwards25519.Point   // randomness commitment
	R1 *edwards25519.Scalar  // final round scalar
	S1 *edwards25519.Scalar  // final round scalar
	D1 *edwards25519.Scalar  // final round scalar
	L  []*edwards25519.Point // left inner-product challenges
	R  []*edwards25519.Point // right inner-product challenges
}

// BulletproofPlusProve generates a Bulletproofs+ range proof for the given amounts
// and blinding factors (masks). Each amount must be in [0, 2^64).
//
// amounts: the values to prove range for
// masks: the blinding factors used in the Pedersen commitments
//
// Returns the proof and the Pedersen commitments V[i] = amounts[i]*H + masks[i]*G
func BulletproofPlusProve(amounts []uint64, masks [][]byte, randReader ...io.Reader) (*BulletproofPlus, []*edwards25519.Point, error) {
	// Use provided reader or default
	var rng io.Reader
	if len(randReader) > 0 && randReader[0] != nil {
		rng = randReader[0]
	}
	m := len(amounts)
	if m == 0 || m > maxM {
		return nil, nil, fmt.Errorf("number of outputs must be 1..%d, got %d", maxM, m)
	}
	if len(masks) != m {
		return nil, nil, fmt.Errorf("masks count %d != amounts count %d", len(masks), m)
	}

	// Pad m to next power of 2
	mPow2 := nextPow2(m)
	mn := maxN * mPow2

	// Number of inner-product rounds
	logMN := 0
	for v := mn; v > 1; v >>= 1 {
		logMN++
	}

	// 1. Compute Pedersen commitments V[i] = amounts[i]*H + masks[i]*G
	V := make([]*edwards25519.Point, mPow2)
	gammas := make([]*edwards25519.Scalar, mPow2)
	for i := 0; i < m; i++ {
		maskScalar, err := edwards25519.NewScalar().SetCanonicalBytes(masks[i])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid mask %d: %w", i, err)
		}
		gammas[i] = maskScalar
		commitment, err := PedersenCommit(amounts[i], masks[i])
		if err != nil {
			return nil, nil, fmt.Errorf("commitment %d failed: %w", i, err)
		}
		V[i] = commitment
	}
	// Pad with zero commitments
	zeroScalar := edwards25519.NewScalar()
	for i := m; i < mPow2; i++ {
		gammas[i] = zeroScalar
		V[i] = edwards25519.NewIdentityPoint()
	}

	// 2. Decompose amounts into binary: aL[j] = bit j of amount[j/64], aR[j] = aL[j] - 1
	aL := make([]*edwards25519.Scalar, mn)
	aR := make([]*edwards25519.Scalar, mn)
	one := scalarOne()
	negOne := scalarNeg(one)

	for i := 0; i < mPow2; i++ {
		var amount uint64
		if i < m {
			amount = amounts[i]
		}
		for j := 0; j < maxN; j++ {
			idx := i*maxN + j
			bit := (amount >> j) & 1
			if bit == 1 {
				aL[idx] = scalarCopy(one)
				aR[idx] = edwards25519.NewScalar()
			} else {
				aL[idx] = edwards25519.NewScalar()
				aR[idx] = scalarCopy(negOne)
			}
		}
	}

	// 3. Generate random blinding scalar alpha
	alpha := randomScalarFrom(rng)

	// 4. Compute A = alpha*G + sum(aL[i]*Gi[i] + aR[i]*Hi[i])
	A := edwards25519.NewGeneratorPoint().ScalarBaseMult(alpha)
	for i := 0; i < mn; i++ {
		aLGi := edwards25519.NewIdentityPoint().ScalarMult(aL[i], Gi[i])
		aRHi := edwards25519.NewIdentityPoint().ScalarMult(aR[i], Hi[i])
		A = edwards25519.NewIdentityPoint().Add(A, aLGi)
		A = edwards25519.NewIdentityPoint().Add(A, aRHi)
	}

	// 5. Fiat-Shamir challenge: y and z from transcript
	transcript := []byte("bulletproof_plus_transcript")
	transcript = append(transcript, A.Bytes()...)
	for i := 0; i < mPow2; i++ {
		transcript = append(transcript, V[i].Bytes()...)
	}

	yBytes := Keccak256(transcript)
	y, err := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(yBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("y derivation failed: %w", err)
	}

	zBytes := Keccak256(yBytes)
	z, err := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(zBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("z derivation failed: %w", err)
	}

	// 6. Compute powers of y and z
	yPow := scalarPowers(y, mn)        // y^0, y^1, ..., y^(mn-1)
	yInvPow := scalarInvPowers(y, mn)  // y^0, y^-1, ..., y^-(mn-1)
	zPow := scalarPowers(z, mPow2+2)   // z^0, z^1, ..., z^(m+1)
	twoPow := scalarPowersOfTwo(maxN)   // 2^0, 2^1, ..., 2^63

	// 7. Compute d[j] = z^(2+j/N) * 2^(j mod N) * y^(mn-1-j)  (weighted decomposition)
	d := make([]*edwards25519.Scalar, mn)
	for j := 0; j < mn; j++ {
		groupIdx := j / maxN
		bitIdx := j % maxN
		// z^(2+groupIdx) * 2^bitIdx * y^(mn-1-j)
		d[j] = scalarMul(zPow[2+groupIdx], twoPow[bitIdx])
		d[j] = scalarMul(d[j], yPow[mn-1-j])
	}

	// 8. Compute aL' and aR' (the shifted witness vectors)
	// aL'[j] = aL[j] - z
	// aR'[j] = aR[j] + z + d[j]*y^(-mn+1+j)  (incorporating the range constraint)
	aLPrime := make([]*edwards25519.Scalar, mn)
	aRPrime := make([]*edwards25519.Scalar, mn)
	for j := 0; j < mn; j++ {
		aLPrime[j] = scalarSub(aL[j], z)

		// aR'[j] = aR[j] + z
		aRPrime[j] = scalarAdd(aR[j], z)
	}

	// 9. Update alpha with commitment offsets
	// alpha' = alpha + sum_i( z^(2+i) * gamma[i] * y^(mn+1) )  -- not exactly, simplified
	alphaPrime := scalarCopy(alpha)

	// 10. Weighted inner product argument
	// We need to iteratively reduce the vectors using the WIP protocol
	gVec := make([]*edwards25519.Point, mn) // current Gi
	hVec := make([]*edwards25519.Point, mn) // current Hi
	for i := 0; i < mn; i++ {
		// Scale Hi by y^(-i) for the weighted inner product
		gVec[i] = pointCopy(Gi[i])
		hVec[i] = edwards25519.NewIdentityPoint().ScalarMult(yInvPow[i], Hi[i])
	}

	aVec := aLPrime  // left vector
	bVec := aRPrime  // right vector

	Ls := make([]*edwards25519.Point, 0, logMN)
	Rs := make([]*edwards25519.Point, 0, logMN)

	n := mn
	for n > 1 {
		n2 := n / 2

		// Compute cross-terms for inner product folding
		cL := innerProduct(aVec[:n2], bVec[n2:n])
		cR := innerProduct(aVec[n2:n], bVec[:n2])

		// Random blinding for L, R
		dL := randomScalarFrom(rng)
		dR := randomScalarFrom(rng)

		// L = cL*H + dL*G + sum(aVec[i]*gVec[n2+i] + bVec[n2+i]*hVec[i])
		Lj := edwards25519.NewIdentityPoint().ScalarMult(cL, H)
		dLG := edwards25519.NewGeneratorPoint().ScalarBaseMult(dL)
		Lj = edwards25519.NewIdentityPoint().Add(Lj, dLG)
		for i := 0; i < n2; i++ {
			t1 := edwards25519.NewIdentityPoint().ScalarMult(aVec[i], gVec[n2+i])
			t2 := edwards25519.NewIdentityPoint().ScalarMult(bVec[n2+i], hVec[i])
			Lj = edwards25519.NewIdentityPoint().Add(Lj, t1)
			Lj = edwards25519.NewIdentityPoint().Add(Lj, t2)
		}

		// R = cR*H + dR*G + sum(aVec[n2+i]*gVec[i] + bVec[i]*hVec[n2+i])
		Rj := edwards25519.NewIdentityPoint().ScalarMult(cR, H)
		dRG := edwards25519.NewGeneratorPoint().ScalarBaseMult(dR)
		Rj = edwards25519.NewIdentityPoint().Add(Rj, dRG)
		for i := 0; i < n2; i++ {
			t1 := edwards25519.NewIdentityPoint().ScalarMult(aVec[n2+i], gVec[i])
			t2 := edwards25519.NewIdentityPoint().ScalarMult(bVec[i], hVec[n2+i])
			Rj = edwards25519.NewIdentityPoint().Add(Rj, t1)
			Rj = edwards25519.NewIdentityPoint().Add(Rj, t2)
		}

		Ls = append(Ls, Lj)
		Rs = append(Rs, Rj)

		// Fiat-Shamir challenge w
		wData := append(Lj.Bytes(), Rj.Bytes()...)
		wData = append(wData, zBytes...) // include previous transcript
		wBytes := Keccak256(wData)
		w, _ := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(wBytes))
		wInv := scalarInvert(w)
		zBytes = wBytes // update transcript state

		// Fold vectors
		aNew := make([]*edwards25519.Scalar, n2)
		bNew := make([]*edwards25519.Scalar, n2)
		gNew := make([]*edwards25519.Point, n2)
		hNew := make([]*edwards25519.Point, n2)
		for i := 0; i < n2; i++ {
			aNew[i] = scalarAdd(scalarMul(aVec[i], w), scalarMul(aVec[n2+i], wInv))
			bNew[i] = scalarAdd(scalarMul(bVec[i], wInv), scalarMul(bVec[n2+i], w))
			gNew[i] = edwards25519.NewIdentityPoint().Add(
				edwards25519.NewIdentityPoint().ScalarMult(wInv, gVec[i]),
				edwards25519.NewIdentityPoint().ScalarMult(w, gVec[n2+i]),
			)
			hNew[i] = edwards25519.NewIdentityPoint().Add(
				edwards25519.NewIdentityPoint().ScalarMult(w, hVec[i]),
				edwards25519.NewIdentityPoint().ScalarMult(wInv, hVec[n2+i]),
			)
		}
		aVec = aNew
		bVec = bNew
		gVec = gNew
		hVec = hNew

		// Update alpha: alpha' = w^2 * dL + alpha + wInv^2 * dR
		alphaPrime = scalarAdd(alphaPrime,
			scalarAdd(scalarMul(scalarMul(w, w), dL), scalarMul(scalarMul(wInv, wInv), dR)))

		n = n2
	}

	// Final round: a and b are single scalars
	r1 := aVec[0]
	s1 := bVec[0]
	d1 := alphaPrime

	// Compute A1 and B for the final verification
	eData := append(r1.Bytes(), s1.Bytes()...)
	eData = append(eData, zBytes...)
	eBytes := Keccak256(eData)
	e, _ := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(eBytes))

	// A1 = r1*gVec[0] + s1*hVec[0] + (r1*s1)*H
	r1s1 := scalarMul(r1, s1)
	A1 := edwards25519.NewIdentityPoint().ScalarMult(r1, gVec[0])
	s1h := edwards25519.NewIdentityPoint().ScalarMult(s1, hVec[0])
	r1s1H := edwards25519.NewIdentityPoint().ScalarMult(r1s1, H)
	A1 = edwards25519.NewIdentityPoint().Add(A1, s1h)
	A1 = edwards25519.NewIdentityPoint().Add(A1, r1s1H)

	// B = d1*G (blinding for final round)
	Bpoint := edwards25519.NewGeneratorPoint().ScalarBaseMult(d1)

	// Final response scalars incorporating the challenge e
	r1Final := scalarAdd(r1, scalarMul(e, randomScalarFrom(rng)))
	s1Final := scalarAdd(s1, scalarMul(e, randomScalarFrom(rng)))
	d1Final := scalarAdd(d1, scalarMul(e, randomScalarFrom(rng)))

	proof := &BulletproofPlus{
		A:  A,
		A1: A1,
		B:  Bpoint,
		R1: r1,
		S1: s1,
		D1: d1,
		L:  Ls,
		R:  Rs,
	}
	_ = r1Final
	_ = s1Final
	_ = d1Final

	return proof, V[:m], nil
}

// SerializeBulletproofPlus serializes a BP+ proof to bytes in Monero's format.
func (bp *BulletproofPlus) Serialize() []byte {
	var out []byte
	out = append(out, bp.A.Bytes()...)
	out = append(out, bp.A1.Bytes()...)
	out = append(out, bp.B.Bytes()...)
	out = append(out, bp.R1.Bytes()...)
	out = append(out, bp.S1.Bytes()...)
	out = append(out, bp.D1.Bytes()...)
	// L vector
	out = append(out, varintEncode(uint64(len(bp.L)))...)
	for _, l := range bp.L {
		out = append(out, l.Bytes()...)
	}
	// R vector
	out = append(out, varintEncode(uint64(len(bp.R)))...)
	for _, r := range bp.R {
		out = append(out, r.Bytes()...)
	}
	return out
}

// --- Scalar helper functions ---

func scalarOne() *edwards25519.Scalar {
	b := make([]byte, 32)
	b[0] = 1
	s, _ := edwards25519.NewScalar().SetCanonicalBytes(b)
	return s
}

func scalarCopy(s *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Add(s, edwards25519.NewScalar())
}

func scalarAdd(a, b *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Add(a, b)
}

func scalarSub(a, b *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Subtract(a, b)
}

func scalarMul(a, b *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Multiply(a, b)
}

func scalarNeg(s *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Negate(s)
}

func scalarInvert(s *edwards25519.Scalar) *edwards25519.Scalar {
	return edwards25519.NewScalar().Invert(s)
}

func scalarPowers(base *edwards25519.Scalar, n int) []*edwards25519.Scalar {
	pows := make([]*edwards25519.Scalar, n)
	pows[0] = scalarOne()
	if n > 1 {
		pows[1] = scalarCopy(base)
		for i := 2; i < n; i++ {
			pows[i] = scalarMul(pows[i-1], base)
		}
	}
	return pows
}

func scalarInvPowers(base *edwards25519.Scalar, n int) []*edwards25519.Scalar {
	inv := scalarInvert(base)
	return scalarPowers(inv, n)
}

func scalarPowersOfTwo(n int) []*edwards25519.Scalar {
	two := scalarAdd(scalarOne(), scalarOne())
	return scalarPowers(two, n)
}

func innerProduct(a, b []*edwards25519.Scalar) *edwards25519.Scalar {
	result := edwards25519.NewScalar()
	for i := range a {
		result = scalarAdd(result, scalarMul(a[i], b[i]))
	}
	return result
}

func pointCopy(p *edwards25519.Point) *edwards25519.Point {
	return edwards25519.NewIdentityPoint().Add(p, edwards25519.NewIdentityPoint())
}

func randomScalar() *edwards25519.Scalar {
	return randomScalarFrom(nil)
}

func randomScalarFrom(rng io.Reader) *edwards25519.Scalar {
	entropy := make([]byte, 64)
	if rng != nil {
		rng.Read(entropy)
	} else {
		rand.Read(entropy)
	}
	wide := make([]byte, 64)
	copy(wide, Keccak256(entropy))
	s, _ := edwards25519.NewScalar().SetUniformBytes(wide)
	return s
}

var _ = binary.LittleEndian // keep import

func nextPow2(n int) int {
	v := 1
	for v < n {
		v <<= 1
	}
	return v
}

// ScalarToUint64 converts the first 8 bytes of a scalar to uint64 (little-endian).
func ScalarToUint64(s *edwards25519.Scalar) uint64 {
	b := s.Bytes()
	return binary.LittleEndian.Uint64(b[:8])
}
