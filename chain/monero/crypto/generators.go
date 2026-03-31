package crypto

import (
	"github.com/cordialsys/crosschain/chain/monero/crypto/cref"
	"filippo.io/edwards25519"
)

// Generator points for Pedersen commitments and Bulletproofs+.
//
// G = Ed25519 base point (blinding factors)
// H = hash_to_ec(G_compressed) (amount commitments)
// C = v*H + r*G

const (
	maxN  = 64  // bits in range proof
	maxM  = 16  // max aggregated outputs
	maxMN = maxN * maxM
)

var H *edwards25519.Point
var Gi [maxMN]*edwards25519.Point
var Hi [maxMN]*edwards25519.Point

func init() {
	// H is a precomputed constant in Monero: the secondary generator for Pedersen commitments.
	// H = toPoint(cn_fast_hash(G)) but using Monero's specific derivation (hardcoded in crypto-ops-data.c)
	hBytes := cref.GetH()
	H, _ = edwards25519.NewIdentityPoint().SetBytes(hBytes[:])

	// Gi and Hi vectors for BP+
	prefix := []byte("bulletproof_plus")
	for i := 0; i < maxMN; i++ {
		hiData := append(prefix, varintEncode(uint64(2*i))...)
		giData := append(prefix, varintEncode(uint64(2*i+1))...)
		hiBytes := HashToEC(hiData)
		giBytes := HashToEC(giData)
		Hi[i], _ = edwards25519.NewIdentityPoint().SetBytes(hiBytes)
		Gi[i], _ = edwards25519.NewIdentityPoint().SetBytes(giBytes)
	}
}

// HashToEC computes Monero's hash_to_ec:
// Keccak256(data) -> ge_fromfe_frombytes_vartime -> multiply by cofactor 8 -> compress
func HashToEC(data []byte) []byte {
	kHash := Keccak256(data)
	result := cref.HashToEC(kHash)
	return result[:]
}

// HashToPoint computes ge_fromfe_frombytes_vartime WITHOUT cofactor multiply.
func HashToPoint(data []byte) []byte {
	result := cref.HashToPointRaw(data)
	return result[:]
}

// ScReduce32 reduces a 32-byte value mod the ed25519 group order L.
// This is Monero's sc_reduce32, NOT the 64-byte SetUniformBytes reduction.
func ScReduce32(s []byte) []byte {
	if len(s) != 32 {
		// Pad or truncate to 32
		buf := make([]byte, 32)
		copy(buf, s)
		s = buf
	}
	result := cref.ScReduce32(s)
	return result[:]
}

// PedersenCommit computes C = v*H + r*G
func PedersenCommit(amount uint64, mask []byte) (*edwards25519.Point, error) {
	vBytes := ScalarFromUint64(amount)
	vScalar, err := edwards25519.NewScalar().SetCanonicalBytes(vBytes)
	if err != nil {
		return nil, err
	}
	vH := edwards25519.NewIdentityPoint().ScalarMult(vScalar, H)

	rScalar, err := edwards25519.NewScalar().SetCanonicalBytes(mask)
	if err != nil {
		return nil, err
	}
	rG := edwards25519.NewGeneratorPoint().ScalarBaseMult(rScalar)

	return edwards25519.NewIdentityPoint().Add(vH, rG), nil
}

// ScalarFromUint64 converts uint64 to 32-byte LE scalar.
func ScalarFromUint64(v uint64) []byte {
	b := make([]byte, 32)
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * i))
	}
	return b
}

// RandomScalar generates a random scalar mod L using Keccak256 of entropy.
func RandomScalar(entropy []byte) []byte {
	hash := Keccak256(entropy)
	return ScReduce32(hash)
}
