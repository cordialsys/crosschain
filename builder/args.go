package builder

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"
)

// All possible builder arguments go in here, privately available.
// Then the public BuilderArgs can typecast and select which arguments are needed.
type builderOptions struct {
	memo           *string
	timestamp      *int64
	gasFeePriority *xc.GasFeePriority
	publicKey      *[]byte

	validator    *string
	stakeOwner   *xc.Address
	stakeAccount *string
}

// All ArgumentBuilders should provide base arguments for transactions
type TransactionOptions interface {
	GetMemo() (string, bool)
	GetTimestamp() (int64, bool)
	GetPriority() (xc.GasFeePriority, bool)
	GetPublicKey() ([]byte, bool)
}

var _ TransactionOptions = &builderOptions{}

func get[T any](arg *T) (T, bool) {
	if arg == nil {
		var zero T
		return zero, false
	}
	return *arg, true
}

// Transaction options
func (opts *builderOptions) GetMemo() (string, bool)                { return get(opts.memo) }
func (opts *builderOptions) GetTimestamp() (int64, bool)            { return get(opts.timestamp) }
func (opts *builderOptions) GetPriority() (xc.GasFeePriority, bool) { return get(opts.gasFeePriority) }
func (opts *builderOptions) GetPublicKey() ([]byte, bool)           { return get(opts.publicKey) }

// Other options
func (opts *builderOptions) GetValidator() (string, bool)      { return get(opts.validator) }
func (opts *builderOptions) GetStakeOwner() (xc.Address, bool) { return get(opts.stakeOwner) }
func (opts *builderOptions) GetStakeAccount() (string, bool)   { return get(opts.stakeAccount) }

type BuilderOption func(opts *builderOptions) error

func OptionMemo(memo string) BuilderOption {
	return func(opts *builderOptions) error {
		opts.memo = &memo
		return nil
	}
}
func OptionTimestamp(ts int64) BuilderOption {
	return func(opts *builderOptions) error {
		opts.timestamp = &ts
		return nil
	}
}
func OptionPriority(priority xc.GasFeePriority) BuilderOption {
	return func(opts *builderOptions) error {
		opts.gasFeePriority = &priority
		return nil
	}
}
func OptionPublicKey(publicKey []byte) BuilderOption {
	return func(opts *builderOptions) error {
		opts.publicKey = &publicKey
		return nil
	}
}

// Set an alternative owner of the stake from the from address
func OptionStakeOwner(owner xc.Address) BuilderOption {
	return func(opts *builderOptions) error {
		opts.stakeOwner = &owner
		return nil
	}
}
func OptionValidator(validator string) BuilderOption {
	return func(opts *builderOptions) error {
		opts.validator = &validator
		return nil
	}
}
func OptionStakeAccount(account string) BuilderOption {
	return func(opts *builderOptions) error {
		opts.stakeAccount = &account
		return nil
	}
}

// Previously the crosschain abstraction would require callers to set options
// directly on the transaction input, if the interface was implemented on the input type.
// However, this is very clear or easy to use.  This function bridges the gap, to allow
// callers to use a more natural interface with options.  Chain transaction builders can
// call this to safely set provided options on the old transaction input setters.
func SetTxInputOptions(txInput xc.TxInput, options TransactionOptions, amount xc.AmountBlockchain) {
	if priority, ok := options.GetPriority(); ok && priority != "" {
		err := txInput.SetGasFeePriority(priority)
		if err != nil {
			logrus.WithError(err).Error("failed to set gas fee priority")
		}
	}
	if pubkey, ok := options.GetPublicKey(); ok {
		if withPubkey, ok := txInput.(xc.TxInputWithPublicKey); ok {
			withPubkey.SetPublicKey(pubkey)
		}
	}

	if withAmount, ok := txInput.(xc.TxInputWithAmount); ok {
		withAmount.SetAmount(amount)
	}
	if memo, ok := options.GetMemo(); ok {
		if withMemo, ok := txInput.(xc.TxInputWithMemo); ok {
			withMemo.SetMemo(memo)
		}
	}
	if timeStamp, ok := options.GetTimestamp(); ok {
		if withUnix, ok := txInput.(xc.TxInputWithUnix); ok {
			withUnix.SetUnix(timeStamp)
		}
	}
}
