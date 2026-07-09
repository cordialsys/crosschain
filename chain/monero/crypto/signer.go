package crypto

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// SignCLSAGFromPayload handles both phases of Monero signing:
// Phase 1 (no RingKeys): derives one-time key → returns key image (32 bytes)
// Phase 2 (has RingKeys): produces full CLSAG ring signature
//
// The signer derives the one-time output key P = H_s(8·v·R‖i)·G + A from its
// own keys and locates it in the ring; the payload never carries the output key
// or the real index. privateViewKey is required (independent of the spend key
// in this implementation).
func SignCLSAGFromPayload(payload []byte, privateSpendKey, privateViewKey []byte) ([]byte, error) {
	var sh MoneroSighash
	if err := json.Unmarshal(payload, &sh); err != nil {
		return nil, fmt.Errorf("failed to decode sighash: %w", err)
	}
	if len(privateViewKey) != 32 {
		return nil, fmt.Errorf("monero signing requires a 32-byte private view key")
	}

	// Derive keys
	privSpendReduced := ScalarReduce(privateSpendKey)
	privSpend, _ := edwards25519.NewScalar().SetCanonicalBytes(privSpendReduced)
	privView := privateViewKey

	// Derive one-time private key and its public key P = H_s(8·v·R‖i)·G + A.
	txPubKeyBytes, _ := hex.DecodeString(sh.TxPubKey)
	derivation, err := GenerateKeyDerivation(txPubKeyBytes, privView)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}
	scalar, _ := DerivationToScalar(derivation, sh.OutputIndex)
	hsScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	oneTimePrivKey := edwards25519.NewScalar().Add(hsScalar, privSpend)
	oneTimePubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(oneTimePrivKey)

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

	// Locate our output in the ring (the signer is told the ring, not which
	// member is real). Absence means the coordinator built a ring around an
	// output this wallet does not own.
	realPos := -1
	for i, member := range ring {
		if member.Equal(oneTimePubKey) == 1 {
			realPos = i
			break
		}
	}
	if realPos < 0 {
		return nil, fmt.Errorf("derived output key is not present in the ring")
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

	// Deterministic nonces à la RFC 6979: seeded from the (secret) one-time key
	// and the message, so signing is reproducible without ever exposing a seed
	// on the wire. A given (key, message) always yields the same signature;
	// distinct transactions yield distinct nonces.
	clsagRng := newDetRNG(Keccak256(append(oneTimePrivKey.Bytes(), sh.Message...)))

	clsag, err := CLSAGSign(&CLSAGContext{
		Message:     sh.Message,
		Ring:        ring,
		CNonzero:    cNonzero,
		COffset:     cOffset,
		SecretIndex: realPos,
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

// MoneroSighash is intentionally duplicated from tx/sighash.go to avoid a
// circular import between the crypto and tx packages. Both definitions must
// be kept in sync.
type MoneroSighash struct {
	Message         []byte   `json:"message,omitempty"`
	RingKeys        []string `json:"ring_keys,omitempty"`
	RingCommitments []string `json:"ring_commitments,omitempty"`
	COffset         string   `json:"c_offset,omitempty"`
	ZKey            string   `json:"z_key,omitempty"`
	TxPubKey        string   `json:"tx_pub_key"`
	OutputIndex     uint64   `json:"output_index"`
}
