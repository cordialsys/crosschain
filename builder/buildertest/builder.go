package buildertest

// Convenient constructors for used in tests

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
)

func DefaultTransferOpts(
	from xc.Address,
	to xc.Address,
	amount xc.AmountBlockchain,
	options ...builder.BuilderOption,
) builder.TransferArgs {
	opts := []builder.BuilderOption{
		// set a max fee bigger than any amount used in tests (effectively disabling by default)
		builder.OptionMaxFees(
			builder.NewNativeMaxFee(xc.NewAmountBlockchainFromStr("10000000000000000000000000000")),
		),
	}
	opts = append(opts, options...)
	return MustNewTransferArgs(from, to, amount, opts...)
}

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

func DefaultStakingOpts(
	chain xc.NativeAsset,
	from xc.Address,
	amount xc.AmountBlockchain,
	options ...builder.BuilderOption,
) builder.StakeArgs {
	opts := []builder.BuilderOption{
		// set a max fee bigger than any amount used in tests (effectively disabling by default)
		builder.OptionMaxFees(
			builder.NewNativeMaxFee(xc.NewAmountBlockchainFromStr("10000000000000000000000000000")),
		),
	}
	opts = append(opts, options...)
	return MustNewStakingArgs(chain, from, amount, opts...)
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
