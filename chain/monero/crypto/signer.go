package crypto

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// SignCLSAGFromPayload handles both phases of Monero signing:
// Phase 1 (no Message/RingKeys): derives one-time key → returns key image (32 bytes)
// Phase 2 (has Message/RingKeys): produces full CLSAG ring signature
func SignCLSAGFromPayload(payload []byte, privateSpendKey []byte) ([]byte, error) {
	var sh MoneroSighash
	if err := json.Unmarshal(payload, &sh); err != nil {
		return nil, fmt.Errorf("failed to decode sighash: %w", err)
	}

	// Derive keys
	privSpendReduced := ScalarReduce(privateSpendKey)
	privSpend, _ := edwards25519.NewScalar().SetCanonicalBytes(privSpendReduced)
	privView := DeriveViewKey(privSpendReduced)

	// Derive one-time private key
	txPubKeyBytes, _ := hex.DecodeString(sh.TxPubKey)
	derivation, err := GenerateKeyDerivation(txPubKeyBytes, privView)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}
	scalar, _ := DerivationToScalar(derivation, sh.OutputIndex)
	hsScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	oneTimePrivKey := edwards25519.NewScalar().Add(hsScalar, privSpend)
	oneTimePubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(oneTimePrivKey)

	// Verify against output key
	outKeyBytes, _ := hex.DecodeString(sh.OutputKey)
	outKeyPt, _ := edwards25519.NewIdentityPoint().SetBytes(outKeyBytes)
	if oneTimePubKey.Equal(outKeyPt) != 1 {
		return nil, fmt.Errorf("derived key does not match output public key")
	}

	keyImage := ComputeKeyImage(oneTimePrivKey, oneTimePubKey)

	// Phase 1: no ring data → return just the key image (32 bytes)
	if len(sh.RingKeys) == 0 {
		return keyImage.Bytes(), nil
	}

	// Phase 2: full CLSAG signing
	ring := make([]*edwards25519.Point, len(sh.RingKeys))
	for i, k := range sh.RingKeys {
		b, _ := hex.DecodeString(k)
		ring[i], _ = edwards25519.NewIdentityPoint().SetBytes(b)
	}

	cNonzero := make([]*edwards25519.Point, len(sh.RingCommitments))
	for i, c := range sh.RingCommitments {
		b, _ := hex.DecodeString(c)
		cNonzero[i], _ = edwards25519.NewIdentityPoint().SetBytes(b)
	}

	cOffsetBytes, _ := hex.DecodeString(sh.COffset)
	cOffset, _ := edwards25519.NewIdentityPoint().SetBytes(cOffsetBytes)

	zKeyBytes, _ := hex.DecodeString(sh.ZKey)
	zKey, _ := edwards25519.NewScalar().SetCanonicalBytes(zKeyBytes)

	var clsagRng io.Reader
	if len(sh.RngSeed) > 0 {
		clsagRng = newDetRNG(sh.RngSeed)
	}

	clsag, err := CLSAGSign(&CLSAGContext{
		Message:     sh.Message,
		Ring:        ring,
		CNonzero:    cNonzero,
		COffset:     cOffset,
		SecretIndex: sh.RealPos,
		SecretKey:   oneTimePrivKey,
		ZKey:        zKey,
		Rand:        clsagRng,
	})
	if err != nil {
		return nil, fmt.Errorf("CLSAG sign failed: %w", err)
	}

	return SerializeCLSAGWithKeyImage(clsag, keyImage), nil
}

// newDetRNG creates a deterministic reader from a seed.
func newDetRNG(seed []byte) io.Reader {
	return &detRNG{state: Keccak256(seed)}
}

type detRNG struct {
	state []byte
	count uint64
}

func (r *detRNG) Read(p []byte) (int, error) {
	for i := 0; i < len(p); i += 32 {
		data := append(r.state, VarIntEncode(r.count)...)
		r.count++
		chunk := Keccak256(data)
		end := i + 32
		if end > len(p) { end = len(p) }
		copy(p[i:end], chunk[:end-i])
	}
	return len(p), nil
}

type MoneroSighash struct {
	Message         []byte   `json:"message"`
	RingKeys        []string `json:"ring_keys"`
	RingCommitments []string `json:"ring_commitments"`
	COffset         string   `json:"c_offset"`
	RealPos         int      `json:"real_pos"`
	ZKey            string   `json:"z_key"`
	OutputKey       string   `json:"output_key"`
	TxPubKey        string   `json:"tx_pub_key"`
	OutputIndex     uint64   `json:"output_index"`
	RngSeed         []byte   `json:"rng_seed,omitempty"`
}
