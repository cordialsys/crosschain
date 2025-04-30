package builder

import (
	xc "github.com/cordialsys/crosschain"
)

// All possible builder arguments go in here, privately available.
// Then the public BuilderArgs can typecast and select which arguments are needed.
type builderOptions struct {
	memo           *string
	timestamp      *int64
	gasFeePriority *xc.GasFeePriority
	// avoiding use of map to ensure determinism in iteration and thread safety
	publicKey         *[]byte
	feePayerPublicKey *[]byte

	validator    *string
	stakeOwner   *xc.Address
	stakeAccount *string
	// asset contract address
	contract *xc.ContractAddress
	decimals *int

	feePayer             *xc.Address
	inclusiveFeeSpending bool
}

func newBuilderOptions() builderOptions {
	return builderOptions{}
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
func (opts *builderOptions) GetMemo() (string, bool)                 { return get(opts.memo) }
func (opts *builderOptions) GetTimestamp() (int64, bool)             { return get(opts.timestamp) }
func (opts *builderOptions) GetPriority() (xc.GasFeePriority, bool)  { return get(opts.gasFeePriority) }
func (opts *builderOptions) GetPublicKey() ([]byte, bool)            { return get(opts.publicKey) }
func (opts *builderOptions) GetContract() (xc.ContractAddress, bool) { return get(opts.contract) }
func (opts *builderOptions) GetDecimals() (int, bool)                { return get(opts.decimals) }
func (opts *builderOptions) GetFeePayer() (xc.Address, bool)         { return get(opts.feePayer) }
func (opts *builderOptions) GetFeePayerPublicKey() ([]byte, bool)    { return get(opts.feePayerPublicKey) }

// Other options
func (opts *builderOptions) GetValidator() (string, bool)      { return get(opts.validator) }
func (opts *builderOptions) GetStakeOwner() (xc.Address, bool) { return get(opts.stakeOwner) }
func (opts *builderOptions) GetStakeAccount() (string, bool)   { return get(opts.stakeAccount) }
func (opts *builderOptions) InclusiveFeeSpendingEnabled() bool {
	return opts.inclusiveFeeSpending
}

func (opts *builderOptions) SetContract(contract xc.ContractAddress) {
	opts.contract = &contract
}

func (opts *builderOptions) SetDecimals(decimals int) {
	opts.decimals = &decimals
}

func (opts *builderOptions) SetFeePayer(feePayer xc.Address) {
	opts.feePayer = &feePayer
}

func (opts *builderOptions) SetFeePayerPublicKey(feePayerPublicKey []byte) {
	opts.feePayerPublicKey = &feePayerPublicKey
}

func (opts *builderOptions) SetInclusiveFeeSpending(inclusiveFeeSpending bool) {
	opts.inclusiveFeeSpending = inclusiveFeeSpending
}

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
func OptionContractAddress(contract xc.ContractAddress, decimalsMaybe ...int) BuilderOption {
	return func(opts *builderOptions) error {
		opts.contract = &contract
		if len(decimalsMaybe) > 0 {
			opts.decimals = &decimalsMaybe[0]
		}
		return nil
	}
}
func OptionContractDecimals(decimals int) BuilderOption {
	return func(opts *builderOptions) error {
		opts.decimals = &decimals
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
func OptionFeePayer(feePayer xc.Address, feePayerPublicKey []byte) BuilderOption {
	return func(opts *builderOptions) error {
		opts.feePayer = &feePayer
		opts.feePayerPublicKey = &feePayerPublicKey
		return nil
	}
}

func OptionInclusiveFeeSpending(inclusiveFeeSpending bool) BuilderOption {
	return func(opts *builderOptions) error {
		opts.inclusiveFeeSpending = inclusiveFeeSpending
		return nil
	}
}

// Previously the crosschain abstraction would require callers to set options
// directly on the transaction input, if the interface was implemented on the input type.
// However, wasn't very clear or easy to use.  This function bridges the gap, to allow
// callers to use a more natural interface with options.  Chain transaction builders can
// call this to safely set provided options on the old transaction input setters.
func WithTxInputOptions(txInput xc.TxInput, timeStamp int64, priorityMaybe xc.GasFeePriority) (xc.TxInput, error) {
	if priorityMaybe != "" {
		err := txInput.SetGasFeePriority(priorityMaybe)
		if err != nil {
			return nil, err
		}
	}
	if withUnix, ok := txInput.(xc.TxInputWithUnix); ok {
		withUnix.SetUnix(timeStamp)
	}
	return txInput, nil
}
