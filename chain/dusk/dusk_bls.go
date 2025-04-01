package dusk

// This package contains utilities to that allows us to follow the signing process used in
// dusk-bls library: https://github.com/dusk-network/bls12_381
import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"github.com/cloudflare/circl/ecc/bls12381"
	"golang.org/x/crypto/blake2b"
)

var R2 = [4]uint64{
	0xc999_e990_f3f2_9c6d,
	0x2b6c_edcb_8792_5c23,
	0x05d3_1496_7254_398f,
	0x0748_d9d9_9f59_ff11,
}

var R3 = [4]uint64{
	0xc62c_1807_439b_73af,
	0x1b3e_0d18_8cf0_6990,
	0x73d1_3c71_c7b5_f418,
	0x6e2a_5bb9_c8db_33e9,
}

// Implements dusk-bls12_381 `hash_to_scalar` function.
// See: https://github.com/dusk-network/bls12_381/blob/master/src/scalar/dusk.rs#L286
func Blake2bScalarReduce(in []byte) (bls12381.Scalar, error) {
	blake2b, err := blake2b.New(64, nil)
	if err != nil {
		return bls12381.Scalar{}, fmt.Errorf("failed to create blake2b hash: %w", err)
	}

	blake2b.Write(in)
	bytes := blake2b.Sum(nil)
	scalarMont1 := [4]uint64{
		binary.LittleEndian.Uint64(bytes[0:8]),
		binary.LittleEndian.Uint64(bytes[8:16]),
		binary.LittleEndian.Uint64(bytes[16:24]),
		binary.LittleEndian.Uint64(bytes[24:32]),
	}
	d1 := ScalarFromMont(scalarMont1)
	r2 := ScalarFromMont(R2)
	var p1 bls12381.Scalar
	p1.Mul(&d1, &r2)

	scalarMont2 := [4]uint64{
		binary.LittleEndian.Uint64(bytes[32:40]),
		binary.LittleEndian.Uint64(bytes[40:48]),
		binary.LittleEndian.Uint64(bytes[48:56]),
		binary.LittleEndian.Uint64(bytes[56:64]),
	}
	d2 := ScalarFromMont(scalarMont2)
	r3 := ScalarFromMont(R3)
	var p2 bls12381.Scalar
	p2.Mul(&d2, &r3)

	var result bls12381.Scalar
	result.Add(&p1, &p2)

	return result, nil
}

// Method `fromMont` from package `ff` in github.com/cloudflare/circl library
// Convert scalar in mont representation to big-endian int representation
func ScalarFromMont(in [4]uint64) bls12381.Scalar {
	var out [4]uint64
	fiatScMontMul(&out, &in, &[4]uint64{1})
	bytesBe := Uint64Le2BytesBe(out[:])
	var scalar bls12381.Scalar
	scalar.SetBytes(bytesBe)
	return scalar
}

// ScalarToLeBytes converts a bls12381.Scalar to a little-endian byte slice.
func ScalarToLeBytes(scalar bls12381.Scalar) ([]byte, error) {
	// MarshalBinary converts scalar to montgomery domain and marshals
	// it in a big-endian order.
	bytes, err := scalar.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scalar: %w", err)
	}

	if len(bytes)%8 != 0 {
		return nil, fmt.Errorf("scalar length is not a multiple of 8")
	}

	// Convert bytes to little-endian order
	leBytes := make([]byte, 0)
	for i := len(bytes); i > 0; i -= 8 {
		u64 := binary.BigEndian.Uint64(bytes[i-8 : i])
		leBytes = binary.LittleEndian.AppendUint64(leBytes, u64)
	}
	return leBytes, nil
}

// This funcion is copied from github.com/cloudflare/circl library.
// For more details browse: https://github.com/cloudflare/circl/blob/main/ecc/bls12381/ff/scalar.go
func fiatScMontMul(out1 *[4]uint64, arg1 *[4]uint64, arg2 *[4]uint64) {
	x1 := arg1[1]
	x2 := arg1[2]
	x3 := arg1[3]
	x4 := arg1[0]
	var x5 uint64
	var x6 uint64
	x6, x5 = bits.Mul64(x4, arg2[3])
	var x7 uint64
	var x8 uint64
	x8, x7 = bits.Mul64(x4, arg2[2])
	var x9 uint64
	var x10 uint64
	x10, x9 = bits.Mul64(x4, arg2[1])
	var x11 uint64
	var x12 uint64
	x12, x11 = bits.Mul64(x4, arg2[0])
	var x13 uint64
	var x14 uint64
	x13, x14 = bits.Add64(x12, x9, uint64(0x0))
	var x15 uint64
	var x16 uint64
	x15, x16 = bits.Add64(x10, x7, uint64(x14))
	var x17 uint64
	var x18 uint64
	x17, x18 = bits.Add64(x8, x5, uint64(x16))
	x19 := (x18 + x6)
	var x20 uint64
	_, x20 = bits.Mul64(x11, 0xfffffffeffffffff)
	var x22 uint64
	var x23 uint64
	x23, x22 = bits.Mul64(x20, 0x73eda753299d7d48)
	var x24 uint64
	var x25 uint64
	x25, x24 = bits.Mul64(x20, 0x3339d80809a1d805)
	var x26 uint64
	var x27 uint64
	x27, x26 = bits.Mul64(x20, 0x53bda402fffe5bfe)
	var x28 uint64
	var x29 uint64
	x29, x28 = bits.Mul64(x20, 0xffffffff00000001)
	var x30 uint64
	var x31 uint64
	x30, x31 = bits.Add64(x29, x26, uint64(0x0))
	var x32 uint64
	var x33 uint64
	x32, x33 = bits.Add64(x27, x24, uint64(x31))
	var x34 uint64
	var x35 uint64
	x34, x35 = bits.Add64(x25, x22, uint64(x33))
	x36 := (x35 + x23)
	var x38 uint64
	_, x38 = bits.Add64(x11, x28, uint64(0x0))
	var x39 uint64
	var x40 uint64
	x39, x40 = bits.Add64(x13, x30, uint64(x38))
	var x41 uint64
	var x42 uint64
	x41, x42 = bits.Add64(x15, x32, uint64(x40))
	var x43 uint64
	var x44 uint64
	x43, x44 = bits.Add64(x17, x34, uint64(x42))
	var x45 uint64
	var x46 uint64
	x45, x46 = bits.Add64(x19, x36, uint64(x44))
	var x47 uint64
	var x48 uint64
	x48, x47 = bits.Mul64(x1, arg2[3])
	var x49 uint64
	var x50 uint64
	x50, x49 = bits.Mul64(x1, arg2[2])
	var x51 uint64
	var x52 uint64
	x52, x51 = bits.Mul64(x1, arg2[1])
	var x53 uint64
	var x54 uint64
	x54, x53 = bits.Mul64(x1, arg2[0])
	var x55 uint64
	var x56 uint64
	x55, x56 = bits.Add64(x54, x51, uint64(0x0))
	var x57 uint64
	var x58 uint64
	x57, x58 = bits.Add64(x52, x49, uint64(x56))
	var x59 uint64
	var x60 uint64
	x59, x60 = bits.Add64(x50, x47, uint64(x58))
	x61 := (x60 + x48)
	var x62 uint64
	var x63 uint64
	x62, x63 = bits.Add64(x39, x53, uint64(0x0))
	var x64 uint64
	var x65 uint64
	x64, x65 = bits.Add64(x41, x55, uint64(x63))
	var x66 uint64
	var x67 uint64
	x66, x67 = bits.Add64(x43, x57, uint64(x65))
	var x68 uint64
	var x69 uint64
	x68, x69 = bits.Add64(x45, x59, uint64(x67))
	var x70 uint64
	var x71 uint64
	x70, x71 = bits.Add64(x46, x61, uint64(x69))
	var x72 uint64
	_, x72 = bits.Mul64(x62, 0xfffffffeffffffff)
	var x74 uint64
	var x75 uint64
	x75, x74 = bits.Mul64(x72, 0x73eda753299d7d48)
	var x76 uint64
	var x77 uint64
	x77, x76 = bits.Mul64(x72, 0x3339d80809a1d805)
	var x78 uint64
	var x79 uint64
	x79, x78 = bits.Mul64(x72, 0x53bda402fffe5bfe)
	var x80 uint64
	var x81 uint64
	x81, x80 = bits.Mul64(x72, 0xffffffff00000001)
	var x82 uint64
	var x83 uint64
	x82, x83 = bits.Add64(x81, x78, uint64(0x0))
	var x84 uint64
	var x85 uint64
	x84, x85 = bits.Add64(x79, x76, uint64(x83))
	var x86 uint64
	var x87 uint64
	x86, x87 = bits.Add64(x77, x74, uint64(x85))
	x88 := (x87 + x75)
	var x90 uint64
	_, x90 = bits.Add64(x62, x80, uint64(0x0))
	var x91 uint64
	var x92 uint64
	x91, x92 = bits.Add64(x64, x82, uint64(x90))
	var x93 uint64
	var x94 uint64
	x93, x94 = bits.Add64(x66, x84, uint64(x92))
	var x95 uint64
	var x96 uint64
	x95, x96 = bits.Add64(x68, x86, uint64(x94))
	var x97 uint64
	var x98 uint64
	x97, x98 = bits.Add64(x70, x88, uint64(x96))
	x99 := (x98 + x71)
	var x100 uint64
	var x101 uint64
	x101, x100 = bits.Mul64(x2, arg2[3])
	var x102 uint64
	var x103 uint64
	x103, x102 = bits.Mul64(x2, arg2[2])
	var x104 uint64
	var x105 uint64
	x105, x104 = bits.Mul64(x2, arg2[1])
	var x106 uint64
	var x107 uint64
	x107, x106 = bits.Mul64(x2, arg2[0])
	var x108 uint64
	var x109 uint64
	x108, x109 = bits.Add64(x107, x104, uint64(0x0))
	var x110 uint64
	var x111 uint64
	x110, x111 = bits.Add64(x105, x102, uint64(x109))
	var x112 uint64
	var x113 uint64
	x112, x113 = bits.Add64(x103, x100, uint64(x111))
	x114 := (x113 + x101)
	var x115 uint64
	var x116 uint64
	x115, x116 = bits.Add64(x91, x106, uint64(0x0))
	var x117 uint64
	var x118 uint64
	x117, x118 = bits.Add64(x93, x108, uint64(x116))
	var x119 uint64
	var x120 uint64
	x119, x120 = bits.Add64(x95, x110, uint64(x118))
	var x121 uint64
	var x122 uint64
	x121, x122 = bits.Add64(x97, x112, uint64(x120))
	var x123 uint64
	var x124 uint64
	x123, x124 = bits.Add64(x99, x114, uint64(x122))
	var x125 uint64
	_, x125 = bits.Mul64(x115, 0xfffffffeffffffff)
	var x127 uint64
	var x128 uint64
	x128, x127 = bits.Mul64(x125, 0x73eda753299d7d48)
	var x129 uint64
	var x130 uint64
	x130, x129 = bits.Mul64(x125, 0x3339d80809a1d805)
	var x131 uint64
	var x132 uint64
	x132, x131 = bits.Mul64(x125, 0x53bda402fffe5bfe)
	var x133 uint64
	var x134 uint64
	x134, x133 = bits.Mul64(x125, 0xffffffff00000001)
	var x135 uint64
	var x136 uint64
	x135, x136 = bits.Add64(x134, x131, uint64(0x0))
	var x137 uint64
	var x138 uint64
	x137, x138 = bits.Add64(x132, x129, uint64(x136))
	var x139 uint64
	var x140 uint64
	x139, x140 = bits.Add64(x130, x127, uint64(x138))
	x141 := (x140 + x128)
	var x143 uint64
	_, x143 = bits.Add64(x115, x133, uint64(0x0))
	var x144 uint64
	var x145 uint64
	x144, x145 = bits.Add64(x117, x135, uint64(x143))
	var x146 uint64
	var x147 uint64
	x146, x147 = bits.Add64(x119, x137, uint64(x145))
	var x148 uint64
	var x149 uint64
	x148, x149 = bits.Add64(x121, x139, uint64(x147))
	var x150 uint64
	var x151 uint64
	x150, x151 = bits.Add64(x123, x141, uint64(x149))
	x152 := (x151 + x124)
	var x153 uint64
	var x154 uint64
	x154, x153 = bits.Mul64(x3, arg2[3])
	var x155 uint64
	var x156 uint64
	x156, x155 = bits.Mul64(x3, arg2[2])
	var x157 uint64
	var x158 uint64
	x158, x157 = bits.Mul64(x3, arg2[1])
	var x159 uint64
	var x160 uint64
	x160, x159 = bits.Mul64(x3, arg2[0])
	var x161 uint64
	var x162 uint64
	x161, x162 = bits.Add64(x160, x157, uint64(0x0))
	var x163 uint64
	var x164 uint64
	x163, x164 = bits.Add64(x158, x155, uint64(x162))
	var x165 uint64
	var x166 uint64
	x165, x166 = bits.Add64(x156, x153, uint64(x164))
	x167 := (x166 + x154)
	var x168 uint64
	var x169 uint64
	x168, x169 = bits.Add64(x144, x159, uint64(0x0))
	var x170 uint64
	var x171 uint64
	x170, x171 = bits.Add64(x146, x161, uint64(x169))
	var x172 uint64
	var x173 uint64
	x172, x173 = bits.Add64(x148, x163, uint64(x171))
	var x174 uint64
	var x175 uint64
	x174, x175 = bits.Add64(x150, x165, uint64(x173))
	var x176 uint64
	var x177 uint64
	x176, x177 = bits.Add64(x152, x167, uint64(x175))
	var x178 uint64
	_, x178 = bits.Mul64(x168, 0xfffffffeffffffff)
	var x180 uint64
	var x181 uint64
	x181, x180 = bits.Mul64(x178, 0x73eda753299d7d48)
	var x182 uint64
	var x183 uint64
	x183, x182 = bits.Mul64(x178, 0x3339d80809a1d805)
	var x184 uint64
	var x185 uint64
	x185, x184 = bits.Mul64(x178, 0x53bda402fffe5bfe)
	var x186 uint64
	var x187 uint64
	x187, x186 = bits.Mul64(x178, 0xffffffff00000001)
	var x188 uint64
	var x189 uint64
	x188, x189 = bits.Add64(x187, x184, uint64(0x0))
	var x190 uint64
	var x191 uint64
	x190, x191 = bits.Add64(x185, x182, uint64(x189))
	var x192 uint64
	var x193 uint64
	x192, x193 = bits.Add64(x183, x180, uint64(x191))
	x194 := (x193 + x181)
	var x196 uint64
	_, x196 = bits.Add64(x168, x186, uint64(0x0))
	var x197 uint64
	var x198 uint64
	x197, x198 = bits.Add64(x170, x188, uint64(x196))
	var x199 uint64
	var x200 uint64
	x199, x200 = bits.Add64(x172, x190, uint64(x198))
	var x201 uint64
	var x202 uint64
	x201, x202 = bits.Add64(x174, x192, uint64(x200))
	var x203 uint64
	var x204 uint64
	x203, x204 = bits.Add64(x176, x194, uint64(x202))
	x205 := (x204 + x177)
	var x206 uint64
	var x207 uint64
	x206, x207 = bits.Sub64(x197, 0xffffffff00000001, uint64(uint64(0x0)))
	var x208 uint64
	var x209 uint64
	x208, x209 = bits.Sub64(x199, 0x53bda402fffe5bfe, uint64(x207))
	var x210 uint64
	var x211 uint64
	x210, x211 = bits.Sub64(x201, 0x3339d80809a1d805, uint64(x209))
	var x212 uint64
	var x213 uint64
	x212, x213 = bits.Sub64(x203, 0x73eda753299d7d48, uint64(x211))
	var x215 uint64
	_, x215 = bits.Sub64(x205, uint64(0x0), uint64(x213))
	var x216 uint64
	fiatScMontCmovznzU64(&x216, x215, x206, x197)
	var x217 uint64
	fiatScMontCmovznzU64(&x217, x215, x208, x199)
	var x218 uint64
	fiatScMontCmovznzU64(&x218, x215, x210, x201)
	var x219 uint64
	fiatScMontCmovznzU64(&x219, x215, x212, x203)
	out1[0] = x216
	out1[1] = x217
	out1[2] = x218
	out1[3] = x219
}

func fiatScMontCmovznzU64(z *uint64, b, x, y uint64) { cselectU64(z, b, x, y) }
func cselectU64(z *uint64, b, x, y uint64)           { *z = (x &^ (-b)) | (y & (-b)) }

// Uint64Le2BytesBe converts a little-endian slice x to a big-endian slice of bytes.
func Uint64Le2BytesBe(x []uint64) []byte {
	b := make([]byte, 8*len(x))
	n := len(x)
	for i := 0; i < n; i++ {
		binary.BigEndian.PutUint64(b[i*8:], x[n-1-i])
	}
	return b
}

// This is a simple double-and-add implementation of point
// multiplication, moving from most significant to least
// significant bit of the scalar.
//
// We skip the leading bit because it's always unset for Fq
// elements.
func ScalarMultShorti(k []byte, P *bls12381.G1) *bls12381.G1 {
	// Since the scalar is short and low Hamming weight not much helps.
	var Q bls12381.G1
	Q.SetIdentity()

	for i := len(k) - 1; i >= 0; i-- {
		byte := k[i]
		for j := 7; j >= 0; j-- {
			if i == len(k)-1 && j == 7 {
				continue // Skip the leading bit
			}
			bit := (byte >> j) & 1

			Q.Double()
			if bit == 1 {
				Q.Add(&Q, P)
			}
		}
	}

	return &Q
}

// scalarMultShort multiplies by a short, constant scalar k, where k is the
// scalar in big-endian order. Runtime depends on the scalar.
func ScalarMultShort(k []byte, P *bls12381.G1) *bls12381.G1 {
	// Since the scalar is short and low Hamming weight not much helps.
	var Q bls12381.G1
	Q.SetIdentity()
	N := 8 * len(k)
	for i := 0; i < N; i++ {
		Q.Double()
		bit := 0x1 & (k[i/8] >> uint(7-i%8))
		if bit != 0 {
			Q.Add(&Q, P)
		}
	}
	return &Q
}
