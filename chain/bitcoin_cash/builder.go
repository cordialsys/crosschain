package bitcoin_cash

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
)

// TxBuilder for Bitcoin
type TxBuilder struct {
	bitcoin.TxBuilder
}

var _ xc.TxBuilder = &TxBuilder{}

// NewTxBuilder creates a new Bitcoin TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	txBuilder, err := bitcoin.NewTxBuilder(cfgI)
	if err != nil {
		return txBuilder, err
	}
	return TxBuilder{
		TxBuilder: txBuilder.(bitcoin.TxBuilder).WithAddressDecoder(&BchAddressDecoder{}),
	}, nil
}

func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txObj, err := txBuilder.TxBuilder.NewTransfer(from, to, amount, input)
	if err != nil {
		return txObj, err
	}
	return txObj.(*tx.Tx), nil
}

func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txObj, err := txBuilder.TxBuilder.NewNativeTransfer(from, to, amount, input)
	if err != nil {
		return txObj, err
	}
	return txObj.(*tx.Tx), nil
}

func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txObj, err := txBuilder.TxBuilder.NewTokenTransfer(from, to, amount, input)
	if err != nil {
		return txObj, err
	}
	return txObj.(*tx.Tx), nil
}
