package substrate

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	xc "github.com/cordialsys/crosschain"
)

// How many blocks the transaction will stay valid for
const MORTAL_PERIOD = 4096

var DefaultMaxTotalTipHuman, _ = xc.NewAmountHumanReadableFromStr("2")

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	sender, err := types.NewMultiAddressFromAccountID(base58.Decode(string(from))[1:33])
	if err != nil {
		return &Tx{}, err
	}
	receiver, err := types.NewMultiAddressFromAccountID(base58.Decode(string(to))[1:33])
	if err != nil {
		return &Tx{}, err
	}

	// We use transfer_keep_alive to avoid accounts being reaped for sending too much balance that it no longer has the
	// existential deposit. This would cause the account to get reaped, which can cause future TXs to have duped hashes
	call, err := NewCall(&txInput.Meta, "Balances.transfer_keep_alive", receiver, types.NewUCompactFromUInt(amount.Uint64()))
	if err != nil {
		return &Tx{}, err
	}

	tip := txInput.Tip
	maxTip := DefaultMaxTotalTipHuman.ToBlockchain(txBuilder.Asset.GetChain().Decimals).Uint64()
	if tip > maxTip {
		tip = maxTip
	}

	return NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, tip, txInput)
}
