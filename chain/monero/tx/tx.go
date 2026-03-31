package tx

import (
	"encoding/hex"
	"errors"

	xc "github.com/cordialsys/crosschain"
	moneroCrypto "github.com/cordialsys/crosschain/chain/monero/crypto"
)

// Tx represents a Monero transaction
type Tx struct {
	// Raw serialized transaction bytes
	TxBlob []byte `json:"tx_blob"`
	// Transaction hash
	TxHash string `json:"tx_hash"`
	// Transaction metadata (JSON from wallet RPC)
	TxMetadata string `json:"tx_metadata,omitempty"`

	// For signing flow: the data that needs to be signed
	SignData []byte `json:"sign_data,omitempty"`
	// The signature(s) collected
	Signatures [][]byte `json:"signatures,omitempty"`
}

func (tx *Tx) Hash() xc.TxHash {
	if tx.TxHash != "" {
		return xc.TxHash(tx.TxHash)
	}
	if len(tx.TxBlob) > 0 {
		hash := moneroCrypto.Keccak256(tx.TxBlob)
		return xc.TxHash(hex.EncodeToString(hash))
	}
	return ""
}

func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.SignData) == 0 {
		return nil, errors.New("no sign data available")
	}
	return []*xc.SignatureRequest{
		{
			Payload: tx.SignData,
		},
	}, nil
}

func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(sigs) == 0 {
		return errors.New("no signatures provided")
	}
	for _, sig := range sigs {
		tx.Signatures = append(tx.Signatures, sig.Signature)
	}
	return nil
}

func (tx *Tx) Serialize() ([]byte, error) {
	if len(tx.TxBlob) > 0 {
		return tx.TxBlob, nil
	}
	return nil, errors.New("transaction not yet constructed")
}
