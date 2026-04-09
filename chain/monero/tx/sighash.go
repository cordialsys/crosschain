package tx

import (
	"encoding/json"
)

// MoneroSighash encodes the CLSAG context into the SignatureRequest payload.
// The Monero signer decodes this to produce the CLSAG ring signature.
//
// NOTE: This struct is intentionally duplicated in crypto/signer.go to avoid
// a circular import between the tx and crypto packages. Both definitions must
// be kept in sync.
type MoneroSighash struct {
	// The CLSAG message hash (32 bytes)
	Message []byte `json:"message"`
	// Ring member public keys (hex)
	RingKeys []string `json:"ring_keys"`
	// Ring member commitments (hex)
	RingCommitments []string `json:"ring_commitments"`
	// Pseudo-output commitment (hex)
	COffset string `json:"c_offset"`
	// Position of real output in the ring
	RealPos int `json:"real_pos"`
	// Commitment mask difference: z = input_mask - pseudo_mask (hex scalar)
	ZKey string `json:"z_key"`
	// Output's one-time public key (hex) - for key derivation
	OutputKey string `json:"output_key"`
	// Original tx public key R (hex) - for key derivation
	TxPubKey string `json:"tx_pub_key"`
	// Output index in original tx
	OutputIndex uint64 `json:"output_index"`
	// RngSeed for deterministic CLSAG nonce generation
	RngSeed []byte `json:"rng_seed,omitempty"`
}

func EncodeSighash(sh *MoneroSighash) []byte {
	data, _ := json.Marshal(sh)
	return data
}

func DecodeSighash(data []byte) (*MoneroSighash, error) {
	var sh MoneroSighash
	err := json.Unmarshal(data, &sh)
	return &sh, err
}
