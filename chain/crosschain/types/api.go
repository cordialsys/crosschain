package types

import (
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
	From string `json:"from"`
	To   string `json:"to"`
}

type TxInputRes struct {
	*TxInputReq
	xc.TxInput `json:"raw_tx_input,omitempty"`
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
	TxData []byte `json:"tx_data"`
}

type SubmitTxRes struct {
	*SubmitTxReq
}

var _ xc.Tx = &SubmitTxReq{}

func (tx *SubmitTxReq) Hash() xc.TxHash {
	panic("not implemented")
}
func (tx *SubmitTxReq) Sighashes() ([]xc.TxDataToSign, error) {
	panic("not implemented")
}
func (tx *SubmitTxReq) AddSignatures(...xc.TxSignature) error {
	panic("not implemented")
}
func (tx *SubmitTxReq) Serialize() ([]byte, error) {
	return tx.TxData, nil
}
