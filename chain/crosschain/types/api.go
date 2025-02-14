package types

import (
	"encoding/json"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"google.golang.org/genproto/googleapis/rpc/status"
)

type Status status.Status

type ApiResponse interface{}

type ChainReq struct {
	Chain string `json:"chain"`
}

type AssetReq struct {
	*ChainReq
	Asset    string `json:"asset,omitempty"`
	Contract string `json:"contract,omitempty"`
	Decimals string `json:"decimals,omitempty"`
}

type BalanceReq struct {
	*AssetReq
	Address string `json:"address"`
}

type BalanceRes struct {
	*BalanceReq
	Balance    xc.AmountHumanReadable `json:"balance"`
	BalanceRaw xc.AmountBlockchain    `json:"balance_raw"`
}

type TxInputReq struct {
	*AssetReq
	From    string `json:"from"`
	To      string `json:"to"`
	Balance string `json:"balance"`
}

type StakingInputReq struct {
	From      string             `json:"from"`
	Balance   string             `json:"balance"`
	Validator string             `json:"validator,omitempty"`
	Account   string             `json:"account,omitempty"`
	Provider  xc.StakingProvider `json:"provider,omitempty"`
}

type LegacyTxInputRes struct {
	*TxInputReq
	xc.TxInput `json:"raw_tx_input,omitempty"`
	NewTxInput json.RawMessage `json:"input,omitempty"`
}

type TxInputRes struct {
	*TxInputReq
	TxInput string `json:"input,omitempty"`
}

type TxInfoReq struct {
	*AssetReq
	TxHash string `json:"tx_hash"`
}

type TxLegacyInfoRes struct {
	*TxInfoReq
	xc.LegacyTxInfo `json:"tx_info,omitempty"`
}

type TransactionInfoRes struct {
	xclient.TxInfo
}

type SubmitTxReq struct {
	*ChainReq
	TxData       []byte   `json:"tx_data"`
	TxSignatures [][]byte `json:"tx_signatures"`
}

type SubmitTxRes struct {
	*SubmitTxReq
}

var _ xc.Tx = &SubmitTxReq{}

func NewBinaryTx(serializedSignedTx []byte, TxSignatures [][]byte) xc.Tx {
	return &SubmitTxReq{
		TxData:       serializedSignedTx,
		TxSignatures: TxSignatures,
	}
}

func (tx *SubmitTxReq) Hash() xc.TxHash {
	panic("not implemented")
}
func (tx *SubmitTxReq) Sighashes() ([]xc.TxDataToSign, error) {
	panic("not implemented")
}
func (tx *SubmitTxReq) AddSignatures(sigs ...xc.TxSignature) error {
	for _, sig := range sigs {
		tx.TxSignatures = append(tx.TxSignatures, sig)
	}
	return nil
}
func (tx *SubmitTxReq) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	for _, sig := range tx.TxSignatures {
		sigs = append(sigs, sig)
	}
	return sigs
}
func (tx *SubmitTxReq) Serialize() ([]byte, error) {
	return tx.TxData, nil
}

type BlockResponse struct {
	Hash           string              `json:"hash"`
	Height         xc.AmountBlockchain `json:"height"`
	Time           *string             `json:"time,omitempty"`
	ChainId        string              `json:"chain_id"`
	TransactionIds []string            `json:"transaction_ids"`
}
