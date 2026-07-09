package tx

import (
	"encoding/json"
)

// MoneroSighash encodes the CLSAG context into the SignatureRequest payload.
// The Monero signer decodes this to produce the CLSAG ring signature.
//
// The payload carries only what the signer cannot compute from its own key
// material. Given the (plaintext) view key v and public spend key A, the signer
// derives the one-time output key P = H_s(8·v·R‖i)·G + A itself, so it locates
// the real ring member (and its position) rather than being told — the
// coordinator cannot point it at an output it does not own, nor mislabel the
// real index. This is what makes an MPC signer safe to drive with this payload:
// there is no request-supplied nonce seed to force nonce reuse (a fatal flaw
// for threshold signing), and no output key to substitute.
//
// NOTE: This struct is intentionally duplicated in crypto/signer.go to avoid
// a circular import between the tx and crypto packages. Both definitions must
// be kept in sync.
type MoneroSighash struct {
	// The CLSAG message hash (32 bytes); empty in phase 1.
	Message []byte `json:"message,omitempty"`
	// Ring member public keys (hex); empty selects the phase-1 key-image request.
	RingKeys []string `json:"ring_keys,omitempty"`
	// Ring member commitments (hex)
	RingCommitments []string `json:"ring_commitments,omitempty"`
	// Pseudo-output commitment (hex)
	COffset string `json:"c_offset,omitempty"`
	// Commitment mask difference: z = input_mask - pseudo_mask (hex scalar).
	// The one value the signer genuinely cannot derive (the pseudo-out mask is
	// the sender's secret).
	ZKey string `json:"z_key,omitempty"`
	// Original tx public key R (hex) - for one-time-key derivation
	TxPubKey string `json:"tx_pub_key"`
	// Output index in original tx - for one-time-key derivation
	OutputIndex uint64 `json:"output_index"`
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
