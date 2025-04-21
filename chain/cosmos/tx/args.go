package tx

import (
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
)

type TxArgs struct {
	Memo              string
	FromPublicKey     []byte
	FeePayer          xc.Address
	FeePayerPublicKey []byte
}

func NewTxArgsFromTransferArgs(args xcbuilder.TransferArgs, input *tx_input.TxInput) TxArgs {
	txArgs := TxArgs{}
	txArgs.Memo, _ = args.GetMemo()
	txArgs.FromPublicKey, _ = args.GetPublicKey()
	txArgs.FeePayer, _ = args.GetFeePayer()
	txArgs.FeePayerPublicKey, _ = args.GetFeePayerPublicKey()
	return txArgs
}
func NewTxArgsFromStakingArgs(args xcbuilder.StakeArgs, input *tx_input.TxInput) TxArgs {
	txArgs := TxArgs{}
	txArgs.Memo, _ = args.GetMemo()
	txArgs.FromPublicKey, _ = args.GetPublicKey()
	txArgs.FeePayer, _ = args.GetFeePayer()
	txArgs.FeePayerPublicKey, _ = args.GetFeePayerPublicKey()
	return txArgs
}
