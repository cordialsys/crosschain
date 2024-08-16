package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/validation"
)

type StakeArgs struct {
	options builderOptions
	from    xc.Address
	amount  xc.AmountBlockchain
}

var _ TransactionOptions = &StakeArgs{}

// Staking arguments
func (args *StakeArgs) GetFrom() xc.Address            { return args.from }
func (args *StakeArgs) GetAmount() xc.AmountBlockchain { return args.amount }

// Exposed options
func (args *StakeArgs) GetMemo() (string, bool)                { return args.options.GetMemo() }
func (args *StakeArgs) GetTimestamp() (int64, bool)            { return args.options.GetTimestamp() }
func (args *StakeArgs) GetPriority() (xc.GasFeePriority, bool) { return args.options.GetPriority() }
func (args *StakeArgs) GetPublicKey() ([]byte, bool)           { return args.options.GetPublicKey() }

// Staking options
func (args *StakeArgs) GetValidator() (string, bool)      { return args.options.GetValidator() }
func (args *StakeArgs) GetStakeOwner() (xc.Address, bool) { return args.options.GetStakeOwner() }
func (args *StakeArgs) GetStakeAccount() (string, bool)   { return args.options.GetStakeAccount() }

func NewStakeArgs(chain xc.NativeAsset, from xc.Address, amount xc.AmountBlockchain, options ...BuilderOption) (StakeArgs, error) {
	builderOptions := builderOptions{}
	args := StakeArgs{
		builderOptions,
		from,
		amount,
	}
	for _, opt := range options {
		err := opt(&args.options)
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
