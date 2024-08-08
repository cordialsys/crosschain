package client

import xc "github.com/cordialsys/crosschain"

type StakedBalanceArgs struct {
	from      xc.Address
	validator *string
	account   *string
}
type StakedBalanceOption func(opts *StakedBalanceArgs) error

func (opts *StakedBalanceArgs) GetFrom() xc.Address          { return opts.from }
func (opts *StakedBalanceArgs) GetValidator() (string, bool) { return get(opts.validator) }
func (opts *StakedBalanceArgs) GetAccount() (string, bool)   { return get(opts.account) }

func NewStakeBalanceArgs(from xc.Address, options ...StakedBalanceOption) (StakedBalanceArgs, error) {
	var validator *string
	var account *string
	args := StakedBalanceArgs{
		from,
		validator,
		account,
	}
	for _, opt := range options {
		err := opt(&args)
		if err != nil {
			return args, err
		}
	}
	return args, nil
}

func StakeBalanceOptionValidator(validator string) StakedBalanceOption {
	return func(opts *StakedBalanceArgs) error {
		opts.validator = &validator
		return nil
	}
}

func StakeBalanceOptionAccount(account string) StakedBalanceOption {
	return func(opts *StakedBalanceArgs) error {
		opts.account = &account
		return nil
	}
}

func get[T any](arg *T) (T, bool) {
	if arg == nil {
		var zero T
		return zero, false
	}
	return *arg, true
}
