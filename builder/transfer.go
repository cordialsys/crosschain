package builder

import (
	xc "github.com/cordialsys/crosschain"
)

type TransferArgs struct {
	TxCommonOptions
	from   xc.Address
	to     xc.Address
	amount xc.AmountBlockchain
}

type TransferOption func(opts *TransferArgs) error

func (opts *TransferArgs) GetFrom() xc.Address            { return opts.from }
func (opts *TransferArgs) GetTo() xc.Address              { return opts.to }
func (opts *TransferArgs) GetAmount() xc.AmountBlockchain { return opts.amount }

func NewTransferArgs(from xc.Address, to xc.Address, amount xc.AmountBlockchain, options ...TransferOption) (TransferArgs, error) {
	common := TxCommonOptions{}
	args := TransferArgs{
		common,
		from,
		to,
		amount,
	}
	for _, opt := range options {
		err := opt(&args)
		if err != nil {
			return args, err
		}
	}
	return args, nil
}

func TransferOptionMemo(memo string) TransferOption {
	return func(opts *TransferArgs) error {
		opts.memo = &memo
		return nil
	}
}
func TransferOptionTimestamp(ts int64) TransferOption {
	return func(opts *TransferArgs) error {
		opts.timestamp = &ts
		return nil
	}
}
func TransferOptionPriority(priority xc.GasFeePriority) TransferOption {
	return func(opts *TransferArgs) error {
		opts.gasFeePriority = &priority
		return nil
	}
}
func TransferOptionPublicKey(publicKey []byte) TransferOption {
	return func(opts *TransferArgs) error {
		opts.publicKey = &publicKey
		return nil
	}
}
