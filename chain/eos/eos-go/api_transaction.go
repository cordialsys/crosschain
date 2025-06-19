package eos

import (
	"context"
	"encoding/json"
	"time"
)

type DownloadedTransaction interface {
	Validate() error

	GetBlockNum() uint64
	GetBlockId() string
	GetBlockTime() time.Time

	GetTxId() string

	GetActions() []TxAction
}

type TxAction interface {
	GetId() string
	GetName() string
	GetAccount() string
	GetData() json.RawMessage
	Ok() bool
}

func (api *API) GetTransactionFromAnySupportedEndpoint(ctx context.Context, id string) (out DownloadedTransaction, err error) {

	txV2, err := api.GetTransactionV2(ctx, id)
	if err != nil {
		if apiErr, ok := err.(APIError); ok {
			if apiErr.Code == 404 {
				// try v1 endppoint
				tx2, err := api.GetTransactionV1(ctx, id)
				if err != nil {
					// can't find any endpoint :(
				} else {
					return tx2, nil
				}
			}
		}
		return nil, err
	}

	return txV2, nil
}
