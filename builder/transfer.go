package builder

import (
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
func (args *TransferArgs) GetContract() (xc.ContractAddress, bool) {
	return args.options.GetContract()
}

func (args *TransferArgs) GetFeePayer() (xc.Address, bool) {
	return args.options.GetFeePayer()
}

// Decimals for token contract, which may be needed for token transfers on some chains
func (args *TransferArgs) GetDecimals() (int, bool) {
	return args.options.GetDecimals()
}

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

	return args, nil
}

func (args *TransferArgs) SetAmount(amount xc.AmountBlockchain) {
	args.amount = amount
}

func (args *TransferArgs) SetFrom(from xc.Address) {
	args.from = from
}

func (args *TransferArgs) SetTo(to xc.Address) {
	args.to = to
}

func (args *TransferArgs) SetContract(contract xc.ContractAddress) {
	args.options.SetContract(contract)
}
