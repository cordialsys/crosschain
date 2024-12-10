package builder

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/tx"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
)

var DefaultMaxTotalTipHuman, _ = xc.NewAmountHumanReadableFromStr("2")

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

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
	sender, err := types.NewMultiAddressFromAccountID(base58.Decode(string(from))[1:33])
	if err != nil {
		return &tx.Tx{}, err
	}
	receiver, err := types.NewMultiAddressFromAccountID(base58.Decode(string(to))[1:33])
	if err != nil {
		return &tx.Tx{}, err
	}

	// We use transfer_keep_alive to avoid accounts being reaped for sending too much balance that it no longer has the
	// existential deposit. This would cause the account to get reaped, which can cause future TXs to have duped hashes
	call, err := tx_input.NewCall(&txInput.Meta, "Balances.transfer_keep_alive", receiver, types.NewUCompactFromUInt(amount.Uint64()))
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
