package api

import xc "github.com/cordialsys/crosschain"

type JettonTransfer struct {
	QueryID             string              `json:"query_id"`
	Source              string              `json:"source"`
	Destination         string              `json:"destination"`
	Amount              string              `json:"amount"`
	SourceWallet        string              `json:"source_wallet"`
	JettonMaster        string              `json:"jetton_master"`
	TransactionHash     string              `json:"transaction_hash"`
	TransactionLT       xc.AmountBlockchain `json:"transaction_lt"`
	TransactionNow      int                 `json:"transaction_now"`
	ResponseDestination string              `json:"response_destination"`
	// CustomPayload       interface{} `json:"custom_payload"`
	// ForwardTonAmount    interface{} `json:"forward_ton_amount"`
	// ForwardPayload      interface{} `json:"forward_payload"`
}

type JettonTransfersResponse struct {
	JettonTransfers []JettonTransfer `json:"jetton_transfers"`
}
