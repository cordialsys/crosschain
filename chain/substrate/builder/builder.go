package builder

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/tx"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
)

var DefaultMaxTotalTipHuman, _ = xc.NewAmountHumanReadableFromStr("2")

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xcbuilder.Staking = &TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI xc.ITask) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

// Old transfer interface
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	switch asset := txBuilder.Asset.(type) {
	case *xc.ChainConfig:
		// ok
	case *xc.TokenAssetConfig:
		return nil, fmt.Errorf("NewTransfer not implemented for tokens on substrate yet")
	default:
		return nil, fmt.Errorf("NewTransfer not implemented for %T", asset)
	}
	sender, err := address.DecodeMulti(from)
	if err != nil {
		return &tx.Tx{}, err
	}
	receiver, err := address.DecodeMulti(to)
	if err != nil {
		return &tx.Tx{}, err
	}

	call, err := tx_input.NewCall(&txInput.Meta, "Balances.transfer_keep_alive", receiver, types.NewUCompact(amount.Int()))
	if err != nil {
		return &tx.Tx{}, err
	}

	tip := txInput.Tip
	maxTip := DefaultMaxTotalTipHuman.ToBlockchain(txBuilder.Asset.GetChain().Decimals).Uint64()
	if tip > maxTip {
		tip = maxTip
	}

	return tx.NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, tip, txInput)
}
