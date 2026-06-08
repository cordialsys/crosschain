package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/gagliardetto/solana-go"
)

type CallArgs struct {
	appliedOptions []BuilderOption
	options        builderOptions
}

func NewCallArgs(chain *xc.ChainBaseConfig, options ...BuilderOption) (CallArgs, error) {
	builderOptions := newBuilderOptions()
	appliedOptions := options
	args := CallArgs{
		appliedOptions,
		builderOptions,
	}
	for _, opt := range options {
		err := opt(&args.options)
		if err != nil {
			return args, err
		}
	}

	switch chain.Driver {
	case xc.DriverSolana:
		if nonceAccount, ok := args.options.GetNonceAccount(); ok {
			_, err := solana.PublicKeyFromBase58(nonceAccount)
			if err != nil {
				return args, fmt.Errorf("invalid nonce account: %v", err)
			}
		}
	}

	return args, nil
}

func (args *CallArgs) GetNonceAccount() (string, bool) {
	return args.options.GetNonceAccount()
}
