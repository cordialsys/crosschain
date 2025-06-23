package eos

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
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
				txV1, err := api.GetTransactionV1(ctx, id)
				if err != nil {
					// can't find any endpoint :(
				} else {
					logrus.WithField("tx_id", id).Debug("retrieved transaction using v1 endpoint")
					return txV1, nil
				}
			}
		}
		return nil, err
	}
	logrus.WithField("tx_id", id).Debug("retrieved transaction using v2 endpoint")

	return txV2, nil
}
