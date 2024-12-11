package types

import (
	// "encoding/json"

	xc "github.com/cordialsys/crosschain"
)

const (
	jsonrpcVersion = "2.0"
	requestId      = 0
)

type RPCRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Id      int64       `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
}

type RPCResponse struct {
	Id      int64  `json:"id"`
	Jsonrpc string `json:"jsonrpc"`
	Result  any    `json:"result"`
}

func NewTransactionRequest(params GetTransactionParams) RPCRequest {
	return RPCRequest{
		Method:  "getTransaction",
		Params:  params,
		Id:      requestId,
		Jsonrpc: jsonrpcVersion,
	}
}

//
// [getTransaction documentation]: https://developers.stellar.org/docs/data/rpc/api-reference/methods/getTransaction
type GetTransactionParams struct {
	Hash xc.TxHash `json:"hash"`
}

//
// [getTransaction documentation]: https://developers.stellar.org/docs/data/rpc/api-reference/methods/getTransaction
type GetTransactionResult struct {
	Status                string `json:"status"`
	LatestLedger          int    `json:"latestLedger"`
	LatestLedgerCloseTime string `json:"latestLedgerCloseTime"`
	OldestLedger          int    `json:"oldestLedger"`
	OldestLedgerCloseTime string `json:"oldestLedgerCloseTime"`
	Ledger                int    `json:"ledger,omitempty"`
	CreatedAt             string `json:"createdAt,omitempty"`
	ApplicationOrder      int    `json:"applicationOrder,omitempty"`
	FeeBump               bool   `json:"feeBump,omitempty"`
	// base64 encoded TransactionEnvelopeXDR
	// [TransactionEnvelopeXDR]
	EnvelopeXdr           string `json:"envelopeXdr,omitempty"`
	ResultXdr             string `json:"resultXdr,omitempty"`
	ResultMetaXdr         string `json:"resultMetaXdr,omitempty"`
}
