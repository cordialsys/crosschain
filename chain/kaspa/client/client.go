package client

import (
	"context"
	"errors"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/kaspa/client/rest"
	"github.com/cordialsys/crosschain/chain/kaspa/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

// Client for Template
type Client struct {
	chain    xc.NativeAsset
	client   *rest.Client
	decimals int
}

var _ xclient.Client = &Client{}

// TODO https://github.com/kaspanet/rusty-kaspa/blob/master/rpc/core/src/api/rpc.rs
func NewClient(cfgI xc.ITask) (*Client, error) {
	chain := cfgI.GetChain()
	clientConfig := chain.ChainClientConfig
	client := rest.NewClient(clientConfig.URL, chain.Chain)
	return &Client{chain.Chain, client, int(chain.Decimals)}, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	utxos, err := client.client.GetUtxos([]string{string(args.GetFrom())})
	if err != nil {
		return nil, err
	}
	txInput := tx_input.NewTxInput()
	txInput.Address = args.GetFrom()
	for _, utxo := range utxos {
		if utxo.UtxoEntry.Amount == nil {
			// skip?
			continue
		}
		txInput.Utxos = append(txInput.Utxos, tx_input.Utxo{
			TransactionId: *utxo.Outpoint.TransactionId,
			Index:         *utxo.Outpoint.Index,
			Amount:        xc.NewAmountBlockchainFromStr(*utxo.UtxoEntry.Amount),
		})
	}
	return txInput, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	return errors.New("not implemented")
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, errors.New("not implemented")
}

func (c *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	return xclient.TxInfo{}, errors.New("not implemented")
}

func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	utxos, err := c.client.GetUtxos([]string{string(args.Address())})
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	total := xc.AmountBlockchain{}
	for _, utxo := range utxos {
		if utxo.UtxoEntry.Amount != nil {
			amount := xc.NewAmountBlockchainFromStr(*utxo.UtxoEntry.Amount)
			total = total.Add(&amount)
		}
	}
	return total, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return client.decimals, nil
}
func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	return &xclient.BlockWithTransactions{}, errors.New("not implemented")
}
