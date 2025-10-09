package builder

import (
	"fmt"
	"strconv"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/tx"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
)

// BondExtra represents the PalletNominationPoolsBondExtra enum
// Enum with variants:
// - FreeBalance(u128) - variant 0
// - Rewards - variant 1
type BondExtra struct {
	IsVariant0    bool
	ValueVariant0 types.U128
}

// Encode implements encoding for BondExtra as a SCALE enum
func (b BondExtra) Encode(encoder scale.Encoder) error {
	var err error
	if b.IsVariant0 {
		err = encoder.PushByte(0)
	} else {
		err = encoder.PushByte(1)
	}
	if err != nil {
		return err
	}
	if b.IsVariant0 {
		err = encoder.Encode(b.ValueVariant0)
	}
	return err
}

// Decode implements decoding for BondExtra (not needed but for completeness)
func (b *BondExtra) Decode(decoder scale.Decoder) error {
	tag, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	if tag == 0 {
		b.IsVariant0 = true
		return decoder.Decode(&b.ValueVariant0)
	}
	b.IsVariant0 = false
	return nil
}

// NominationPoolsStakingBuilder handles staking transaction building for substrate chains using nomination pools
type NominationPoolsStakingBuilder struct {
	txBuilder *TxBuilder
}

// NewNominationPoolsStakingBuilder creates a new nomination pools staking builder
func NewNominationPoolsStakingBuilder(txBuilder *TxBuilder) *NominationPoolsStakingBuilder {
	return &NominationPoolsStakingBuilder{
		txBuilder: txBuilder,
	}
}

func (pools *NominationPoolsStakingBuilder) Stake(args xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	amount := args.GetAmount()

	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return &tx.Tx{}, err
	}

	poolIdStr, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("must provide pool ID in validator field")
	}

	// Parse pool ID
	poolId, err := strconv.ParseUint(poolIdStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid pool ID: %s", poolIdStr)
	}

	var call types.Call

	// Check if account already joined a pool
	if txInput.AlreadyJoinedPool {
		// Use bond_extra for accounts already in a pool
		// bond_extra(extra: PalletNominationPoolsBondExtra)
		bondExtra := BondExtra{
			// FreeBalance variant
			IsVariant0:    true,
			ValueVariant0: types.NewU128(*amount.Int()),
		}
		call, err = tx_input.NewCall(&txInput.Meta, "NominationPools.bond_extra",
			bondExtra,
		)
		if err != nil {
			return &tx.Tx{}, err
		}
	} else {
		// Build the NominationPools.join call
		// join(amount: Compact<u128>, pool_id: u32)
		call, err = tx_input.NewCall(&txInput.Meta, "NominationPools.join",
			types.NewUCompact(amount.Int()),
			types.NewU32(uint32(poolId)),
		)
		if err != nil {
			return &tx.Tx{}, err
		}
	}

	tip := txInput.Tip
	maxTip := DefaultMaxTotalTipHuman.ToBlockchain(pools.txBuilder.Asset.Decimals).Uint64()
	if tip > maxTip {
		tip = maxTip
	}

	return tx.NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, tip, txInput)
}

func (pools *NominationPoolsStakingBuilder) Unstake(args xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	amount := args.GetAmount()

	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return &tx.Tx{}, err
	}

	// unbond(member_account: MultiAddress, unbonding_points: Compact<u128>)
	call, err := tx_input.NewCall(&txInput.Meta, "NominationPools.unbond",
		sender,
		types.NewUCompact(amount.Int()),
	)
	if err != nil {
		return &tx.Tx{}, err
	}

	tip := txInput.Tip
	maxTip := DefaultMaxTotalTipHuman.ToBlockchain(pools.txBuilder.Asset.Decimals).Uint64()
	if tip > maxTip {
		tip = maxTip
	}

	return tx.NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, tip, txInput)
}

func (pools *NominationPoolsStakingBuilder) Withdraw(args xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)

	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return &tx.Tx{}, err
	}

	// Build the NominationPools.withdraw_unbonded call
	// withdraw_unbonded(member_account: MultiAddress, num_slashing_spans: u32)
	call, err := tx_input.NewCall(&txInput.Meta, "NominationPools.withdraw_unbonded",
		sender,
		types.NewU32(txInput.NumSlashingSpans),
	)
	if err != nil {
		return &tx.Tx{}, err
	}

	tip := txInput.Tip
	maxTip := DefaultMaxTotalTipHuman.ToBlockchain(pools.txBuilder.Asset.Decimals).Uint64()
	if tip > maxTip {
		tip = maxTip
	}

	return tx.NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, tip, txInput)
}
