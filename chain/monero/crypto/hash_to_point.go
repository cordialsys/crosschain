package crypto

import (
	"filippo.io/edwards25519"
	"filippo.io/edwards25519/field"
)

// Precomputed field element constants for ge_fromfe_frombytes_vartime.
// These match Monero's crypto-ops-data.c values.
var (
	feMA     field.Element
	feMA2    field.Element
	feSqrtM1 field.Element
	feFFfb1  field.Element
	feFFfb2  field.Element
	feFFfb3  field.Element
	feFFfb4  field.Element
	htpInitDone bool
)

// ensureHtpInit lazily initializes the Elligator constants.
// Called before first use to avoid init() ordering issues.
func ensureHtpInit() {
	if htpInitDone {
		return
	}
	htpInitDone = true
	initHtpConstants()
}

func initHtpConstants() {
	// Montgomery A = 486662
	var feA, feTwo, feAp2 field.Element
	feA.SetBytes(uint64ToLE(486662))
	feTwo.SetBytes(uint64ToLE(2))

	// fe_ma = -A
	feMA.Negate(&feA)

	// fe_ma2 = -A^2 (NOT -2*A^2; the factor 2 comes from v in the Elligator map)
	var aSq field.Element
	aSq.Square(&feA)
	feMA2.Negate(&aSq)

	// A + 2
	feAp2.Add(&feA, &feTwo)

	// sqrt(-1) computed via SqrtRatio(-1, 1)
	var negOne field.Element
	negOne.Negate(new(field.Element).One())
	feSqrtM1.SqrtRatio(&negOne, new(field.Element).One())

	// Compute the fffb constants using SqrtRatio
	var one field.Element
	one.One()
	var aAp2 field.Element
	aAp2.Multiply(&feA, &feAp2) // A * (A+2)

	var neg2aAp2, pos2aAp2 field.Element
	pos2aAp2.Multiply(&feTwo, &aAp2) // 2*A*(A+2)
	neg2aAp2.Negate(&pos2aAp2)       // -2*A*(A+2)

	feFFfb1.SqrtRatio(&neg2aAp2, &one) // sqrt(-2*A*(A+2))
	feFFfb2.SqrtRatio(&pos2aAp2, &one) // sqrt(2*A*(A+2))

	var negSqrtM1aAp2, sqrtM1aAp2 field.Element
	sqrtM1aAp2.Multiply(&feSqrtM1, &aAp2) // sqrt(-1)*A*(A+2)
	negSqrtM1aAp2.Negate(&sqrtM1aAp2)     // -sqrt(-1)*A*(A+2)
	feFFfb3.SqrtRatio(&negSqrtM1aAp2, &one) // sqrt(-sqrt(-1)*A*(A+2))
	feFFfb4.SqrtRatio(&sqrtM1aAp2, &one)    // sqrt(sqrt(-1)*A*(A+2))
}

// geFromfeFrombytesVartime implements Monero's ge_fromfe_frombytes_vartime.
// Maps a 32-byte hash to an Edwards curve point (in projective X:Y:Z coordinates).
// This is NOT the standard Ed25519 point decompression - it's an Elligator-like map.
func geFromfeFrombytesVartime(s []byte) *edwards25519.Point {
	ensureHtpInit()
	var u, v, w, x, y, z field.Element

	// u = field element from bytes.
	// Monero's fe_frombytes does NOT reduce mod p or clear the high bit.
	// field.Element.SetBytes clears bit 255. We add 2^255 back if needed.
	highBit := (s[31] >> 7) & 1
	u.SetBytes(s)
	if highBit == 1 {
		// Add 2^255 = 19 (mod p, since p = 2^255 - 19)
		// Actually 2^255 mod p = 19, so we add 19
		var nineteen field.Element
		nineteen.SetBytes(uint64ToLE(19))
		u.Add(&u, &nineteen)
	}

	// v = 2 * u^2
	v.Square(&u)
	var two field.Element
	two.Add(new(field.Element).One(), new(field.Element).One())
	v.Multiply(&v, &two)

	// w = 2*u^2 + 1
	w.Add(&v, new(field.Element).One())

	// x = w^2 - 2*A^2*u^2
	x.Square(&w)
	var ma2v field.Element
	ma2v.Multiply(&feMA2, &v)
	x.Add(&x, &ma2v) // x = w^2 + (-2*A^2)*u^2 = w^2 - 2*A^2*u^2

	// r->X = (w/x)^(m+1) where m = (p-5)/8
	// This is fe_divpowm1(r->X, w, x) = w * x^3 * (w*x^7)^((p-5)/8)
	var rX field.Element
	feDivPowM1(&rX, &w, &x)

	// y = rX^2
	y.Square(&rX)
	// x = y * x (reusing x)
	x.Multiply(&y, &x)

	// Check branches (matching C code exactly)
	var yCheck field.Element
	yCheck.Subtract(&w, &x) // y = w - rX^2*x

	z.Set(&feMA) // z = -A

	sign := 0

	if yCheck.Equal(new(field.Element).Zero()) != 1 {
		// w - rX^2*x != 0
		yCheck.Add(&w, &x) // y = w + rX^2*x
		if yCheck.Equal(new(field.Element).Zero()) != 1 {
			// Both checks failed -> negative branch
			x.Multiply(&x, &feSqrtM1) // x *= sqrt(-1)
			yCheck.Subtract(&w, &x)
			if yCheck.Equal(new(field.Element).Zero()) != 1 {
				// assert(w + x == 0)
				rX.Multiply(&rX, &feFFfb3)
			} else {
				rX.Multiply(&rX, &feFFfb4)
			}
			// z stays as -A, sign = 1
			sign = 1
		} else {
			// w + rX^2*x == 0
			rX.Multiply(&rX, &feFFfb1)
			rX.Multiply(&rX, &u) // u * sqrt(...)
			z.Multiply(&z, &v)   // z = -A * 2u^2 = -2Au^2
			sign = 0
		}
	} else {
		// w - rX^2*x == 0
		rX.Multiply(&rX, &feFFfb2)
		rX.Multiply(&rX, &u) // u * sqrt(...)
		z.Multiply(&z, &v)   // z = -A * 2u^2 = -2Au^2
		sign = 0
	}

	// Set sign
	if rX.IsNegative() != sign {
		rX.Negate(&rX)
	}

	// Projective Edwards coordinates:
	// rZ = z + w
	// rY = z - w
	// rX = rX * rZ
	var rZ, rY field.Element
	rZ.Add(&z, &w)
	rY.Subtract(&z, &w)
	rX.Multiply(&rX, &rZ)

	// Convert from projective (X:Y:Z) to compressed Edwards point
	// Affine: x = X/Z, y = Y/Z
	// Compressed: encode y with sign bit of x
	var invZ field.Element
	invZ.Invert(&rZ)
	var affX, affY field.Element
	affX.Multiply(&rX, &invZ)
	affY.Multiply(&rY, &invZ)

	// Encode as compressed Edwards point: y with high bit = sign of x
	yBytes := affY.Bytes()
	if affX.IsNegative() == 1 {
		yBytes[31] |= 0x80
	}

	point, err := edwards25519.NewIdentityPoint().SetBytes(yBytes)
	if err != nil {
		// Should not happen for valid Elligator output
		return edwards25519.NewIdentityPoint()
	}
	return point
}

// feDivPowM1 computes r = (u/v)^((p+3)/8) = u * v^3 * (u*v^7)^((p-5)/8)
func feDivPowM1(r, u, v *field.Element) {
	var v3, uv7 field.Element
	v3.Square(v)
	v3.Multiply(&v3, v) // v^3
	uv7.Square(&v3)
	uv7.Multiply(&uv7, v)
	uv7.Multiply(&uv7, u) // u*v^7

	var t0 field.Element
	t0.Pow22523(&uv7) // (u*v^7)^((p-5)/8)

	r.Multiply(u, &v3)
	r.Multiply(r, &t0) // u * v^3 * (u*v^7)^((p-5)/8)
}

// hashToPointPureGo computes ge_fromfe_frombytes_vartime WITHOUT cofactor.
func HashToPointPureGo(data []byte) *edwards25519.Point {
	return geFromfeFrombytesVartime(data)
}

// hashToECPureGo computes hash_to_ec: Keccak256 -> Elligator map -> multiply by cofactor 8.
func HashToECPureGo(data []byte) []byte {
	hash := Keccak256(data)
	point := geFromfeFrombytesVartime(hash)

	// Multiply by cofactor 8 (3 doublings)
	p2 := edwards25519.NewIdentityPoint().Add(point, point)
	p4 := edwards25519.NewIdentityPoint().Add(p2, p2)
	p8 := edwards25519.NewIdentityPoint().Add(p4, p4)

	return p8.Bytes()
}

// Helper: convert uint64 to 32-byte little-endian
func uint64ToLE(v uint64) []byte {
	b := make([]byte, 32)
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * i))
	}
	return b
}

// Helper: set field element from LE bytes
func setBytesLE(fe *field.Element, b []byte) {
	fe.SetBytes(b)
}

// Helper: hex string to bytes
func hexBytes(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		h := htpHexVal(s[i])
		l := htpHexVal(s[i+1])
		b[i/2] = byte(h<<4 | l)
	}
	return b, nil
}

func htpHexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	default:
		return 0
	}
}
