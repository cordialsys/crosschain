package types

import (
	// "encoding/json"

	xc "github.com/cordialsys/crosschain"
)

const (
	jsonrpcVersion = "2.0"
	requestId = 0
)

type RPCRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Id      int64       `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
}

func NewTransactionRequest(params GetTransactionParams) *RPCRequest {
	return &RPCRequest{
		Method: "getTransaction",
		Params: params,
		Id: 1,
		Jsonrpc: jsonrpcVersion,
	}
}

type GetTransactionParams struct {
	Hash xc.TxHash `json:"hash"`
}

type TransactionParamEntry struct {
	Transaction xc.TxHash `json:"transaction"`
	Binary      bool      `json:"binary"`
}

