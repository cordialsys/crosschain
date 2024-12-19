package builder

import (
	"fmt"
	"math/big"
	"strings"

	"cosmossdk.io/math"
	stakingtypes "cosmossdk.io/x/staking/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cosmos/cosmos-sdk/types"
)

var _ xc.TxXTransferBuilder = &TxBuilder{}

func (txBuilder TxBuilder) NewTask(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	task := txBuilder.Asset.(*xc.TaskConfig)
	amountInt := big.Int(amount)
	amountCoin := types.Coin{
		Denom:  txBuilder.GetDenom(),
		Amount: math.NewIntFromBigInt(&amountInt),
	}

	if strings.HasPrefix(task.Code, "CosmosUndelegateOperator") {
		validatorAddress, ok := task.DefaultParams["validator_address"]
		if !ok {
			return &tx.Tx{}, fmt.Errorf("must provide validator_address in task '%s'", txBuilder.Asset.ID())
		}
		msgUndelegate := &stakingtypes.MsgUndelegate{
			DelegatorAddress: string(from),
			Amount:           amountCoin,
			ValidatorAddress: fmt.Sprintf("%s", validatorAddress),
		}

		fees := txBuilder.calculateFees(amount, txInput, false)
		return txBuilder.createTxWithMsg(txInput, msgUndelegate, txArgs{
			Memo:          txInput.LegacyMemo,
			FromPublicKey: txInput.LegacyFromPublicKey,
		}, fees)
	}

	return &tx.Tx{}, fmt.Errorf("not implemented task: '%s'", txBuilder.Asset.ID())
}
