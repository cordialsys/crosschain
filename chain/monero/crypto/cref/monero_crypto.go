package cref

// #cgo CFLAGS: -DNDEBUG
// #cgo LDFLAGS: -L${SRCDIR} -lbpplus -lstdc++ -lsodium
// #include "crypto-ops.h"
// #include "bp_plus_wrapper.h"
// #include <string.h>
//
// // get_H: return the precomputed H generator point
// void monero_get_H(unsigned char *result) {
//   ge_p3_tobytes(result, &ge_p3_H);
// }
//
// // hash_to_ec: Keccak256(data) -> ge_fromfe_frombytes_vartime -> mul8 -> compress
// // This matches Monero's hash_to_ec used for key image computation.
// void monero_hash_to_ec(const unsigned char *pubkey, unsigned char *result) {
//   // We receive the already-hashed bytes (Keccak256 output) from Go.
//   // Apply ge_fromfe_frombytes_vartime + cofactor multiply.
//   ge_p2 point_p2;
//   ge_p1p1 point_p1p1;
//   ge_p3 point_p3;
//
//   ge_fromfe_frombytes_vartime(&point_p2, pubkey);
//
//   // Multiply by cofactor 8: p2 -> p1p1 (via mul8) -> p3 -> compress
//   ge_mul8(&point_p1p1, &point_p2);
//   ge_p1p1_to_p3(&point_p3, &point_p1p1);
//   ge_p3_tobytes(result, &point_p3);
// }
//
// // hash_to_point_raw: ge_fromfe_frombytes_vartime WITHOUT cofactor multiply.
// // Used for matching the "hash_to_point" test vectors.
// void monero_hash_to_point_raw(const unsigned char *input, unsigned char *result) {
//   ge_p2 point;
//   ge_fromfe_frombytes_vartime(&point, input);
//   ge_tobytes(result, &point);
// }
//
// // generate_key_derivation: D = 8 * secret * public
// void monero_generate_key_derivation(const unsigned char *pub, const unsigned char *sec, unsigned char *derivation) {
//   ge_p3 pub_point;
//   ge_p2 tmp2;
//   ge_p1p1 tmp1;
//
//   ge_frombytes_vartime(&pub_point, pub);
//   ge_scalarmult(&tmp2, sec, &pub_point);
//   ge_mul8(&tmp1, &tmp2);
//   ge_p1p1_to_p2(&tmp2, &tmp1);
//   ge_tobytes(derivation, &tmp2);
// }
//
// // derivation_to_scalar: H_s(derivation || varint(output_index))
// void monero_derivation_to_scalar(const unsigned char *derivation, unsigned int output_index, unsigned char *scalar) {
//   // Build buffer: derivation (32 bytes) + varint(output_index)
//   unsigned char buf[32 + 10]; // max varint size is 10
//   memcpy(buf, derivation, 32);
//   int len = 32;
//   unsigned int val = output_index;
//   while (val >= 0x80) {
//     buf[len++] = (val & 0x7f) | 0x80;
//     val >>= 7;
//   }
//   buf[len++] = val;
//   // We'll do the Keccak hash in Go since we already have it there.
//   // Just return the raw data for Go to hash.
//   memcpy(scalar, buf, len);
//   // Store length in the last byte position we can use
//   scalar[31] = (unsigned char)len;
// }
//
// // derive_public_key: derived = scalar*G + base
// void monero_derive_public_key(const unsigned char *derivation, unsigned int output_index, const unsigned char *base, unsigned char *derived) {
//   // Compute scalar = H_s(derivation || varint(output_index))
//   unsigned char buf[32 + 10];
//   memcpy(buf, derivation, 32);
//   int len = 32;
//   unsigned int val = output_index;
//   while (val >= 0x80) {
//     buf[len++] = (val & 0x7f) | 0x80;
//     val >>= 7;
//   }
//   buf[len++] = val;
//
//   // The caller will do Keccak in Go and pass us the scalar directly.
//   // For this function, we compute scalar*G + base in the C code.
//   // But we need the scalar from Go... Let's have Go handle the hash part.
//   // This function takes the already-computed scalar.
//   ge_p3 base_point;
//   ge_p3 result;
//   ge_frombytes_vartime(&base_point, base);
//
//   // scalar * G
//   ge_p3 sG;
//   ge_scalarmult_base(&sG, derivation); // reuse derivation as scalar input
//
//   // sG + base
//   ge_cached base_cached;
//   ge_p1p1 sum_p1p1;
//   ge_p3_to_cached(&base_cached, &base_point);
//   ge_add(&sum_p1p1, &sG, &base_cached);
//   ge_p1p1_to_p3(&result, &sum_p1p1);
//   ge_p3_tobytes(derived, &result);
// }
//
// // sc_reduce32: reduce a 32-byte value mod L
// void monero_sc_reduce32(unsigned char *s) {
//   sc_reduce32(s);
// }
//
// // generate_key_image: I = secret * hash_to_ec(public)
// void monero_generate_key_image(const unsigned char *pub, const unsigned char *sec, unsigned char *image) {
//   ge_p2 hp_p2;
//   ge_p1p1 hp_p1p1;
//   ge_p3 hp;
//   ge_p2 result;
//
//   // hash_to_ec(pub) = cofactor * ge_fromfe_frombytes_vartime(Keccak(pub))
//   // The caller passes Keccak(pub) as `pub` here.
//   ge_fromfe_frombytes_vartime(&hp_p2, pub);
//   ge_mul8(&hp_p1p1, &hp_p2);
//   ge_p1p1_to_p3(&hp, &hp_p1p1);
//
//   // I = sec * hp
//   ge_scalarmult(&result, sec, &hp);
//   ge_tobytes(image, &result);
// }
import "C"
import (
	"fmt"
	"unsafe"
)

// GetH returns the precomputed H generator point used for Pedersen commitments.
func GetH() [32]byte {
	var result [32]byte
	C.monero_get_H((*C.uchar)(unsafe.Pointer(&result[0])))
	return result
}

// HashToEC computes Monero's hash_to_ec: ge_fromfe_frombytes_vartime(input) * 8
// Input should be the Keccak256 hash of the public key.
func HashToEC(keccakHash []byte) [32]byte {
	var result [32]byte
	C.monero_hash_to_ec(
		(*C.uchar)(unsafe.Pointer(&keccakHash[0])),
		(*C.uchar)(unsafe.Pointer(&result[0])),
	)
	return result
}

// HashToPointRaw computes ge_fromfe_frombytes_vartime WITHOUT cofactor multiply.
// This matches the "hash_to_point" test vectors in Monero's test suite.
func HashToPointRaw(input []byte) [32]byte {
	var result [32]byte
	C.monero_hash_to_point_raw(
		(*C.uchar)(unsafe.Pointer(&input[0])),
		(*C.uchar)(unsafe.Pointer(&result[0])),
	)
	return result
}

// GenerateKeyDerivation computes D = 8 * secret * public (compressed output).
func GenerateKeyDerivation(pub, sec []byte) [32]byte {
	var result [32]byte
	C.monero_generate_key_derivation(
		(*C.uchar)(unsafe.Pointer(&pub[0])),
		(*C.uchar)(unsafe.Pointer(&sec[0])),
		(*C.uchar)(unsafe.Pointer(&result[0])),
	)
	return result
}

// ScReduce32 reduces a 32-byte value mod the ed25519 group order L.
func ScReduce32(s []byte) [32]byte {
	var result [32]byte
	copy(result[:], s)
	C.monero_sc_reduce32((*C.uchar)(unsafe.Pointer(&result[0])))
	return result
}

// GenerateKeyImage computes I = secret * hash_to_ec(Keccak(public))
// keccakPub should be Keccak256(public_key), sec is the secret scalar.
func GenerateKeyImage(keccakPub, sec []byte) [32]byte {
	var result [32]byte
	C.monero_generate_key_image(
		(*C.uchar)(unsafe.Pointer(&keccakPub[0])),
		(*C.uchar)(unsafe.Pointer(&sec[0])),
		(*C.uchar)(unsafe.Pointer(&result[0])),
	)
	return result
}

// BPPlusProve generates a Bulletproofs+ range proof using Monero's exact C++ implementation.
// amounts: slice of uint64 values to prove range for
// masks: slice of 32-byte blinding factors (one per amount)
// Returns the serialized proof bytes.
func BPPlusProve(amounts []uint64, masks [][]byte) ([]byte, error) {
	if len(amounts) == 0 || len(amounts) != len(masks) {
		return nil, fmt.Errorf("invalid BP+ input: %d amounts, %d masks", len(amounts), len(masks))
	}
	count := len(amounts)

	// Flatten masks into contiguous byte array
	flatMasks := make([]byte, count*32)
	for i, m := range masks {
		if len(m) != 32 {
			return nil, fmt.Errorf("mask %d is %d bytes, expected 32", i, len(m))
		}
		copy(flatMasks[i*32:], m)
	}

	// Output buffer
	proofBuf := make([]byte, 8192)
	proofLen := C.int(len(proofBuf))

	ret := C.monero_bp_plus_prove(
		(*C.uint64_t)(unsafe.Pointer(&amounts[0])),
		(*C.uchar)(unsafe.Pointer(&flatMasks[0])),
		C.int(count),
		(*C.uchar)(unsafe.Pointer(&proofBuf[0])),
		&proofLen,
	)
	if ret != 0 {
		return nil, fmt.Errorf("BP+ prove failed with code %d", ret)
	}

	return proofBuf[:proofLen], nil
}

// BPPlusProveRaw is a lower-level version that returns the raw proof and the V commitments separately.
// Returns (V commitments as [][]byte, proof fields as raw bytes, error)
func BPPlusProveRaw(amounts []uint64, masks [][]byte) ([][]byte, BPPlusFields, error) {
	raw, err := BPPlusProve(amounts, masks)
	if err != nil {
		return nil, BPPlusFields{}, err
	}
	return ParseBPPlusProof(raw)
}

// BPPlusFields contains the parsed fields of a BP+ proof.
type BPPlusFields struct {
	A, A1, B     [32]byte
	R1, S1, D1   [32]byte
	L            [][32]byte
	R            [][32]byte
}

// ParseBPPlusProof parses the serialized proof from the C wrapper.
// Returns (V commitments, proof fields, error).
func ParseBPPlusProof(raw []byte) ([][]byte, BPPlusFields, error) {
	var fields BPPlusFields
	pos := 0

	readU32 := func() uint32 {
		v := uint32(raw[pos]) | uint32(raw[pos+1])<<8 | uint32(raw[pos+2])<<16 | uint32(raw[pos+3])<<24
		pos += 4
		return v
	}
	readKey := func() [32]byte {
		var k [32]byte
		copy(k[:], raw[pos:pos+32])
		pos += 32
		return k
	}

	nV := int(readU32())
	V := make([][]byte, nV)
	for i := 0; i < nV; i++ {
		k := readKey()
		V[i] = k[:]
	}

	fields.A = readKey()
	fields.A1 = readKey()
	fields.B = readKey()
	fields.R1 = readKey()
	fields.S1 = readKey()
	fields.D1 = readKey()

	nL := int(readU32())
	fields.L = make([][32]byte, nL)
	for i := 0; i < nL; i++ {
		fields.L[i] = readKey()
	}

	nR := int(readU32())
	fields.R = make([][32]byte, nR)
	for i := 0; i < nR; i++ {
		fields.R[i] = readKey()
	}

	return V, fields, nil
}

// BPPlusVerify verifies a Bulletproofs+ range proof using Monero's exact C++ implementation.
func BPPlusVerify(proof []byte) bool {
	ret := C.monero_bp_plus_verify(
		(*C.uchar)(unsafe.Pointer(&proof[0])),
		C.int(len(proof)),
	)
	return ret == 1
}
