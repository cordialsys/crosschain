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
