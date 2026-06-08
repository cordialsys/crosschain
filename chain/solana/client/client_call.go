package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/gagliardetto/solana-go"
)

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall, args builder.CallArgs) (xc.CallTxInput, error) {
	fromAddr := call.SigningAddresses()[0]

	var nonceAccountMaybe *solana.PublicKey
	nonceAccount, ok := args.GetNonceAccount()
	if ok {
		nonceAccountPub, err := solana.PublicKeyFromBase58(nonceAccount)
		if err != nil {
			return nil, fmt.Errorf("invalid nonce account: %s: %v", nonceAccount, err)
		}
		nonceAccountMaybe = &nonceAccountPub
	}

	txInput, err := client.FetchBaseInput(ctx, fromAddr, "", xc.NewAmountBlockchainFromUint64(0), nonceAccountMaybe)
	if err != nil {
		return nil, err
	}
	return &tx_input.CallInput{TxInput: *txInput}, nil
}
