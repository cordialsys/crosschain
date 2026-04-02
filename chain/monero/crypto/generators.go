package crypto

import (
	"filippo.io/edwards25519"
	"github.com/cordialsys/crosschain/chain/monero/crypto/cref"
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
	ensureHtpInit() // must be called before using HashToECPureGo

	// H is a precomputed constant from Monero's crypto-ops-data.c
	H, _ = edwards25519.NewIdentityPoint().SetBytes(GetHPureGo())

	// Gi and Hi vectors for BP+ using pure Go hash_to_ec
	prefix := []byte("bulletproof_plus")
	for i := 0; i < maxMN; i++ {
		hiData := make([]byte, len(prefix))
		copy(hiData, prefix)
		hiData = append(hiData, varintEncode(uint64(2*i))...)
		giData := make([]byte, len(prefix))
		copy(giData, prefix)
		giData = append(giData, varintEncode(uint64(2*i+1))...)
		hiBytes := HashToECPureGo(hiData)
		giBytes := HashToECPureGo(giData)
		Hi[i], _ = edwards25519.NewIdentityPoint().SetBytes(hiBytes)
		Gi[i], _ = edwards25519.NewIdentityPoint().SetBytes(giBytes)
	}
}

// HashToEC computes Monero's hash_to_ec:
// Keccak256(data) -> ge_fromfe_frombytes_vartime -> multiply by cofactor 8 -> compress
func HashToEC(data []byte) []byte {
	return HashToECPureGo(data)
}

// HashToPoint computes ge_fromfe_frombytes_vartime WITHOUT cofactor multiply.
func HashToPoint(data []byte) []byte {
	return HashToPointPureGo(data).Bytes()
}

// ScReduce32 reduces a 32-byte value mod the ed25519 group order L.
func ScReduce32(s []byte) []byte {
	return ScReduce32PureGo(s)
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

// BPPlusProveNative generates a BP+ proof using Monero's exact C++ implementation.
// Returns (V commitments, serialized proof fields for tx, prunable hash data, error).
func BPPlusProveNative(amounts []uint64, masks [][]byte) (commitments [][]byte, proofFields cref.BPPlusFields, err error) {
	return cref.BPPlusProveRaw(amounts, masks)
}
