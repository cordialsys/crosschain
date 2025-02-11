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

func (txBuilder TxBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	amount := args.GetAmount()

	validator, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("must provide validator address")
	}
	validatorAddr, err := address.Decode(xc.Address(validator))
	if err != nil {
		return &tx.Tx{}, err
	}
	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return &tx.Tx{}, err
	}

	// must use a types.NewU64
	call, err := tx_input.NewCall(&txInput.Meta, "SubtensorModule.add_stake", validatorAddr, types.NewU64(amount.Uint64()))
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

func (txBuilder TxBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	amount := args.GetAmount()

	validator, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("must provide validator address")
	}
	validatorAddr, err := address.Decode(xc.Address(validator))
	if err != nil {
		return &tx.Tx{}, err
	}
	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return &tx.Tx{}, err
	}

	call, err := tx_input.NewCall(&txInput.Meta, "SubtensorModule.remove_stake", validatorAddr, types.NewU64(amount.Uint64()))
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
func (txBuilder TxBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}
