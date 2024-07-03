package bitcoin_cash

import (
	"errors"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
)

// TxBuilder for Bitcoin
type TxBuilder struct {
	bitcoin.TxBuilder
}

type BchAddressDecoder struct{}

var _ bitcoin.AddressDecoder = &BchAddressDecoder{}
var _ xc.TxBuilder = &TxBuilder{}

func (*BchAddressDecoder) Decode(inputAddr xc.Address, params *chaincfg.Params) (btcutil.Address, error) {
	addr, err := btcutil.DecodeAddress(string(inputAddr), params)
	if err != nil {
		// try to decode as BCH
		bchaddr, err2 := DecodeBchAddress(string(inputAddr), params)
		if err2 != nil {
			return nil, errors.Join(err, err2)
		}
		addr, err2 = BchAddressFromBytes(bchaddr, params)
		if err2 != nil {
			return nil, errors.Join(err, err2)
		}
	}
	return addr, nil
}

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
