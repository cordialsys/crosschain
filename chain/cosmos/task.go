package cosmos

import (
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var _ xc.TxXTransferBuilder = &TxBuilder{}

func (txBuilder TxBuilder) NewTask(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	task := txBuilder.Asset.(*xc.TaskConfig)
	amountInt := big.Int(amount)
	amountCoin := types.Coin{
		Denom:  txBuilder.GetDenom(),
		Amount: types.NewIntFromBigInt(&amountInt),
	}

	if strings.HasPrefix(task.Code, "CosmosUndelegateOperator") {
		validatorAddress, ok := task.DefaultParams["validator_address"]
		if !ok {
			return &Tx{}, fmt.Errorf("must provide validator_address in task '%s'", txBuilder.Asset.ID())
		}
		msgUndelegate := &stakingtypes.MsgUndelegate{
			DelegatorAddress: string(from),
			Amount:           amountCoin,
			ValidatorAddress: fmt.Sprintf("%s", validatorAddress),
		}

		return txBuilder.createTxWithMsg(from, to, amount, txInput, msgUndelegate)
	}

	return &Tx{}, fmt.Errorf("not implemented task: '%s'", txBuilder.Asset.ID())
}
