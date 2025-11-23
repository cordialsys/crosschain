package types

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type SubmitTxReq struct {
	Chain  xc.NativeAsset `json:"chain"`
	TxData []byte         `json:"tx_data"`
	// Left to support older clients still using
	LegacyTxSignatures [][]byte `json:"tx_signatures"`
	// Mapping for Tx "metadata" embedded JSON
	BroadcastInput string `json:"input,omitempty"`
}

var _ xc.Tx = &SubmitTxReq{}
var _ xc.TxWithMetadata = &SubmitTxReq{}
var _ xc.TxLegacyGetSignatures = &SubmitTxReq{}

func (tx *SubmitTxReq) Hash() xc.TxHash {
	panic("not implemented")
}
func (tx *SubmitTxReq) Sighashes() ([]*xc.SignatureRequest, error) {
	panic("not implemented")
}
func (tx *SubmitTxReq) SetSignatures(sigs ...*xc.SignatureResponse) error {
	for _, sig := range sigs {
		tx.LegacyTxSignatures = append(tx.LegacyTxSignatures, sig.Signature)
	}
	return nil
}
func (tx *SubmitTxReq) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	for _, sig := range tx.LegacyTxSignatures {
		sigs = append(sigs, sig)
	}
	return sigs
}
func (tx *SubmitTxReq) Serialize() ([]byte, error) {
	return tx.TxData, nil
}
func (tx *SubmitTxReq) GetMetadata() ([]byte, bool, error) {
	return []byte(tx.BroadcastInput), tx.BroadcastInput != "", nil
}

func SubmitTxReqFromTx(chain xc.NativeAsset, tx xc.Tx) (SubmitTxReq, error) {
	metadata := ""
	if mtx, ok := tx.(xc.TxWithMetadata); ok {
		md, _, err := mtx.GetMetadata()
		if err != nil {
			return SubmitTxReq{}, fmt.Errorf("failed to get tx metadata: %w", err)
		}
		metadata = string(md)
	}

	legacySigs := [][]byte{}
	if mtx, ok := tx.(xc.TxLegacyGetSignatures); ok {
		for _, sig := range mtx.GetSignatures() {
			legacySigs = append(legacySigs, sig)
		}
	}

	txData, err := tx.Serialize()
	if err != nil {
		return SubmitTxReq{}, fmt.Errorf("failed to serialize tx: %w", err)
	}

	return SubmitTxReq{
		Chain:              chain,
		TxData:             txData,
		LegacyTxSignatures: legacySigs,
		BroadcastInput:     metadata,
	}, nil
}

func NewBinaryTx(serializedSignedTx []byte, broadcastInput []byte) xc.Tx {
	return &SubmitTxReq{
		TxData:         serializedSignedTx,
		BroadcastInput: string(broadcastInput),
	}
}
