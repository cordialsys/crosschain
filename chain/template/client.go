package newchain

import (
	"context"
	"errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/utils"
)

// Client for Template
type Client struct {
}

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	utils.TxPriceInput
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{},
	}
}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	return &Client{}, errors.New("not implemented")
}

// FetchTxInput returns tx input for a Template tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	return &TxInput{}, errors.New("not implemented")
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	return errors.New("not implemented")
}

// FetchTxInfo returns tx info for a Template tx
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xc.TxInfo, error) {
	return xc.TxInfo{}, errors.New("not implemented")
}
