package buildertest

// Convenient constructors for used in tests

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
)

func MustNewTransferArgs(
	from xc.Address,
	to xc.Address,
	amount xc.AmountBlockchain,
	options ...builder.BuilderOption,
) builder.TransferArgs {
	args, err := builder.NewTransferArgs(from, to, amount, options...)
	if err != nil {
		panic(err)
	}
	return args
}

func MustNewStakingArgs(
	chain xc.NativeAsset,
	from xc.Address,
	amount xc.AmountBlockchain,
	options ...builder.BuilderOption,
) builder.StakeArgs {
	args, err := builder.NewStakeArgs(chain, from, amount, options...)
	if err != nil {
		panic(err)
	}
	return args
}

// Re-export for convenience
var OptionContractAddress = builder.OptionContractAddress
var OptionContractDecimals = builder.OptionContractDecimals
var OptionValidator = builder.OptionValidator
var OptionStakeOwner = builder.OptionStakeOwner
var OptionStakeAccount = builder.OptionStakeAccount
var OptionTimestamp = builder.OptionTimestamp
var OptionPriority = builder.OptionPriority
var OptionPublicKey = builder.OptionPublicKey
var OptionMemo = builder.OptionMemo
var OptionTxInput = builder.WithTxInputOptions
