package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type TransferArgs struct {
	// TxCommonOptions
	options builderOptions
	from    xc.Address
	to      xc.Address
	amount  xc.AmountBlockchain
}

var _ TransactionOptions = &TransferArgs{}

// Transfer relevant arguments
func (args *TransferArgs) GetFrom() xc.Address            { return args.from }
func (args *TransferArgs) GetTo() xc.Address              { return args.to }
func (args *TransferArgs) GetAmount() xc.AmountBlockchain { return args.amount }

// Exposed options
func (args *TransferArgs) GetMemo() (string, bool)                { return args.options.GetMemo() }
func (args *TransferArgs) GetTimestamp() (int64, bool)            { return args.options.GetTimestamp() }
func (args *TransferArgs) GetPriority() (xc.GasFeePriority, bool) { return args.options.GetPriority() }
func (args *TransferArgs) GetPublicKey() ([]byte, bool)           { return args.options.GetPublicKey() }

func NewTransferArgs(from xc.Address, to xc.Address, amount xc.AmountBlockchain, options ...BuilderOption) (TransferArgs, error) {
	builderOptions := newBuilderOptions()
	args := TransferArgs{
		builderOptions,
		from,
		to,
		amount,
	}
	for _, opt := range options {
		err := opt(&args.options)
		if err != nil {
			return args, err
		}
	}
	if len(args.options.maxFees) == 0 {
		return args, fmt.Errorf("max_fee is a required argument for transactions")
	}

	return args, nil
}
