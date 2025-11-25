package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
)

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall) (xc.CallTxInput, error) {
	fromAddr := call.SigningAddresses()[0]
	txInput, err := client.FetchBaseInput(ctx, fromAddr)
	if err != nil {
		return nil, err
	}
	return &tx_input.CallInput{TxInput: *txInput}, nil
}
