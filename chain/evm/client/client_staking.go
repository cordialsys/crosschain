package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xcclient "github.com/cordialsys/crosschain/client"
)

func (cli *Client) FetchStakeBalance(ctx context.Context, args xcclient.StakedBalanceArgs) ([]*xcclient.StakedBalance, error) {
	validator, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("must provider a validator to lookup balance for")
	}
	validatorBal, err := cli.FetchValidatorBalance(ctx, validator)
	if err != nil {
		return nil, err
	} else {
		return []*xcclient.StakedBalance{validatorBal}, nil
	}
}

func (cli *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	return nil, fmt.Errorf("EVM does not yet natively support delegated staking, must use a 3rd party provider")
}
func (cli *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	return nil, fmt.Errorf("EVM does not yet natively support delegated staking, must use a 3rd party provider")
}
func (cli *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	return nil, fmt.Errorf("EVM does not yet natively support delegated staking, must use a 3rd party provider")
}
