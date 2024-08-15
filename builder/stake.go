package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/validation"
)

type StakeArgs struct {
	TxCommonOptions
	from      xc.Address
	amount    xc.AmountBlockchain
	validator *string
	owner     *xc.Address
	account   *string
}
type StakeOption func(opts *StakeArgs) error

func (opts *StakeArgs) GetFrom() xc.Address            { return opts.from }
func (opts *StakeArgs) GetAmount() xc.AmountBlockchain { return opts.amount }
func (opts *StakeArgs) GetValidator() (string, bool)   { return get(opts.validator) }
func (opts *StakeArgs) GetOwner() (xc.Address, bool)   { return get(opts.owner) }
func (opts *StakeArgs) GetAccount() (string, bool)     { return get(opts.account) }

func NewStakeArgs(chain xc.NativeAsset, from xc.Address, amount xc.AmountBlockchain, options ...StakeOption) (StakeArgs, error) {
	common := TxCommonOptions{}
	var validator *string
	var owner *xc.Address
	var accountId *string
	args := StakeArgs{
		common,
		from,
		amount,
		validator,
		owner,
		accountId,
	}
	for _, opt := range options {
		err := opt(&args)
		if err != nil {
			return args, err
		}
	}

	// Chain specific validation of arguments
	switch chain.Driver() {
	case xc.DriverEVM:
		// Eth must stake or unstake in increments of 32
		_, err := validation.Count32EthChunks(args.GetAmount())
		if err != nil {
			return args, err
		}
	case xc.DriverCosmos, xc.DriverSolana:
		if _, ok := args.GetValidator(); !ok {
			return args, fmt.Errorf("validator to be delegated to is required for %s chain", chain)
		}
	}

	return args, nil
}

func StakeOptionValidator(validator string) StakeOption {
	return func(opts *StakeArgs) error {
		opts.validator = &validator
		return nil
	}
}
func StakeOptionAccount(account string) StakeOption {
	return func(opts *StakeArgs) error {
		opts.account = &account
		return nil
	}
}
func StakeOptionMemo(memo string) StakeOption {
	return func(opts *StakeArgs) error {
		opts.memo = &memo
		return nil
	}
}
func StakeOptionTimestamp(ts int64) StakeOption {
	return func(opts *StakeArgs) error {
		opts.timestamp = &ts
		return nil
	}
}
func StakeOptionPriority(priority xc.GasFeePriority) StakeOption {
	return func(opts *StakeArgs) error {
		opts.gasFeePriority = &priority
		return nil
	}
}
func StakeOptionPublicKey(publicKey []byte) StakeOption {
	return func(opts *StakeArgs) error {
		opts.publicKey = &publicKey
		return nil
	}
}

// Set an alternative owner of the stake from the from address
func StakeOptionOwner(owner xc.Address) StakeOption {
	return func(opts *StakeArgs) error {
		opts.owner = &owner
		return nil
	}
}
