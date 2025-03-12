package bitcoin_cash

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	bitcointx "github.com/cordialsys/crosschain/chain/bitcoin/tx"
)

// TxBuilder for Bitcoin
type TxBuilder struct {
	bitcoin.TxBuilder
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

// NewTxBuilder creates a new Bitcoin TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	txBuilder, err := bitcoin.NewTxBuilder(cfgI)
	if err != nil {
		return TxBuilder{}, err
	}
	return TxBuilder{
		TxBuilder: txBuilder.WithAddressDecoder(&BchAddressDecoder{}),
	}, nil
}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	tx, err := txBuilder.TxBuilder.Transfer(args, input)
	if err != nil {
		return nil, err
	}
	return NewTx(tx.(*bitcointx.Tx)), nil
}
