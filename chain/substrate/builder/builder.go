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

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xcbuilder.Staking = &TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	builder := TxBuilder{
		Asset: cfgI,
	}
	return builder, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if _, ok := args.GetContract(); ok {
		return nil, fmt.Errorf("token transfers not supported on substrate")
	}
	txInput := input.(*tx_input.TxInput)

	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return &tx.Tx{}, err
	}
	receiver, err := address.DecodeMulti(args.GetTo())
	if err != nil {
		return &tx.Tx{}, err
	}

	call, err := tx_input.NewCall(&txInput.Meta, "Balances.transfer_keep_alive", receiver, types.NewUCompact(args.GetAmount().Int()))
	if err != nil {
		return &tx.Tx{}, err
	}

	return tx.NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, txInput.Tip, txInput)
}
