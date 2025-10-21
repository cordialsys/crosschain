package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	buildererrors "github.com/cordialsys/crosschain/builder/errors"
	"github.com/cordialsys/crosschain/builder/validation"
)

type StakeArgs struct {
	options builderOptions
	from    xc.Address
}

var _ TransactionOptions = &StakeArgs{}

// Staking arguments
func (args *StakeArgs) GetFrom() xc.Address { return args.from }
func (args *StakeArgs) GetAmount() (xc.AmountBlockchain, bool) {
	return args.options.GetStakeAmount()
}

// Exposed options
func (args *StakeArgs) GetMemo() (string, bool)                { return args.options.GetMemo() }
func (args *StakeArgs) GetTimestamp() (int64, bool)            { return args.options.GetTimestamp() }
func (args *StakeArgs) GetPriority() (xc.GasFeePriority, bool) { return args.options.GetPriority() }
func (args *StakeArgs) GetPublicKey() ([]byte, bool)           { return args.options.GetPublicKey() }

// Staking options
func (args *StakeArgs) GetValidator() (string, bool)      { return args.options.GetValidator() }
func (args *StakeArgs) GetStakeOwner() (xc.Address, bool) { return args.options.GetStakeOwner() }
func (args *StakeArgs) GetStakeAccount() (string, bool)   { return args.options.GetStakeAccount() }
func (args *StakeArgs) GetFeePayer() (xc.Address, bool)   { return args.options.GetFeePayer() }
func (args *StakeArgs) GetFeePayerPublicKey() ([]byte, bool) {
	return args.options.GetFeePayerPublicKey()
}
func (args *StakeArgs) GetFeePayerIdentity() (string, bool) {
	return args.options.GetFromIdentity()
}
func (args *StakeArgs) GetFromIdentity() (string, bool) { return args.options.GetFromIdentity() }

func NewStakeArgs(chain xc.NativeAsset, from xc.Address, options ...BuilderOption) (StakeArgs, error) {
	builderOptions := builderOptions{}
	args := StakeArgs{
		builderOptions,
		from,
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
		amount, ok := args.GetAmount()
		if !ok {
			return args, buildererrors.ErrStakingAmountRequired
		}
		// Eth must stake or unstake in increments of 32
		_, err := validation.Count32EthChunks(xc.NewAmountBlockchainFromUint64(amount.Uint64()))
		if err != nil {
			return args, err
		}
	case xc.DriverCardano:
		_, ok := args.GetAmount()
		if ok {
			return args, fmt.Errorf("%w: cardano always uses the full balance of the address", buildererrors.ErrStakingAmountNotUsed)
		}
		if _, ok := args.GetValidator(); !ok {
			return args, fmt.Errorf("validator to be delegated to is required for %s chain", chain)
		}
	case xc.DriverCosmos, xc.DriverSolana, xc.DriverSubstrate:
		_, ok := args.GetAmount()
		if !ok {
			return args, buildererrors.ErrStakingAmountRequired
		}

		if _, ok := args.GetValidator(); !ok {
			return args, fmt.Errorf("validator to be delegated to is required for %s chain", chain)
		}
	default:
		_, ok := args.GetAmount()
		if !ok {
			return args, buildererrors.ErrStakingAmountRequired
		}
	}

	return args, nil
}
