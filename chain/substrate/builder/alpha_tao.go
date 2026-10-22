package builder

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/substrate/address"
	"github.com/cordialsys/crosschain/chain/substrate/tx"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
)

// TaoAlphaBuilder handles alpha token transfer building for Bittensor (TAO) chain
type TaoAlphaBuilder struct {
	txBuilder *TxBuilder
}

// NewTaoAlphaBuilder creates a new TAO alpha token builder
func NewTaoAlphaBuilder(txBuilder *TxBuilder) *TaoAlphaBuilder {
	return &TaoAlphaBuilder{
		txBuilder: txBuilder,
	}
}

// TransferStake builds one or more SubtensorModule.transfer_stake extrinsics to transfer
// alpha tokens from one coldkey to another. Positions are selected UTXO-style from the
// sender's alpha holdings across hotkeys on the target subnet.
func (b *TaoAlphaBuilder) TransferStake(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)

	contract, ok := args.GetContract()
	if !ok {
		return nil, fmt.Errorf("contract address required for alpha transfer")
	}

	netuid, err := substrate.ParseAlphaContract(contract)
	if err != nil {
		return nil, err
	}

	sender, err := address.DecodeMulti(args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("invalid sender address: %v", err)
	}

	destinationColdkey, err := address.Decode(args.GetTo())
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %v", err)
	}

	if len(txInput.AlphaPositions) == 0 {
		return nil, fmt.Errorf("no alpha positions available on subnet %d", netuid)
	}

	transferAmount := args.GetAmount().Uint64()
	if transferAmount == 0 {
		return nil, fmt.Errorf("transfer amount must be greater than zero")
	}

	// Select positions UTXO-style (positions are pre-sorted descending by amount)
	calls, err := b.selectAndBuildCalls(txInput, destinationColdkey, netuid, transferAmount)
	if err != nil {
		return nil, err
	}

	var call types.Call
	if len(calls) == 1 {
		// Single position covers the transfer — no batching needed
		call = calls[0]
	} else {
		// Multiple positions needed — wrap in Utility.batch_all
		call, err = b.buildBatchCall(txInput, calls)
		if err != nil {
			return nil, err
		}
	}

	return tx.NewTx(extrinsic.NewDynamicExtrinsic(&call), sender, txInput.Tip, txInput)
}

// selectAndBuildCalls picks alpha positions to cover the transfer amount and builds
// individual transfer_stake calls for each.
func (b *TaoAlphaBuilder) selectAndBuildCalls(
	txInput *tx_input.TxInput,
	destinationColdkey *types.AccountID,
	netuid uint16,
	remaining uint64,
) ([]types.Call, error) {
	originNetuid := types.NewU16(netuid)
	destinationNetuid := types.NewU16(netuid)

	var calls []types.Call
	for _, pos := range txInput.AlphaPositions {
		if remaining == 0 {
			break
		}

		hotkey, err := address.Decode(xc.Address(pos.Hotkey))
		if err != nil {
			return nil, fmt.Errorf("invalid hotkey address in position: %v", err)
		}

		spend := pos.Amount
		if spend > remaining {
			spend = remaining
		}

		call, err := tx_input.NewCall(
			&txInput.Meta,
			"SubtensorModule.transfer_stake",
			destinationColdkey,
			hotkey,
			originNetuid,
			destinationNetuid,
			types.NewU64(spend),
		)
		if err != nil {
			return nil, err
		}

		calls = append(calls, call)
		remaining -= spend
	}

	if remaining > 0 {
		return nil, fmt.Errorf("insufficient alpha balance: still need %d more", remaining)
	}

	return calls, nil
}

// buildBatchCall wraps multiple calls into a Utility.batch_all call.
// batch_all takes a Vec<Call>, encoded as a SCALE compact-length-prefixed array.
func (b *TaoAlphaBuilder) buildBatchCall(txInput *tx_input.TxInput, calls []types.Call) (types.Call, error) {
	// Encode each call individually, then build the Vec<Call> encoding
	var encodedCalls []byte

	// SCALE Vec prefix: compact-encoded length
	lenPrefix, err := codec.Encode(types.NewUCompactFromUInt(uint64(len(calls))))
	if err != nil {
		return types.Call{}, fmt.Errorf("failed to encode batch length: %v", err)
	}
	encodedCalls = append(encodedCalls, lenPrefix...)

	for _, call := range calls {
		encoded, err := codec.Encode(call)
		if err != nil {
			return types.Call{}, fmt.Errorf("failed to encode call for batch: %v", err)
		}
		encodedCalls = append(encodedCalls, encoded...)
	}

	// Look up the Utility.batch_all call index
	callIndex, err := txInput.Meta.FindCallIndex("Utility.batch_all")
	if err != nil {
		return types.Call{}, err
	}

	return types.Call{
		CallIndex: callIndex,
		Args:      encodedCalls,
	}, nil
}
