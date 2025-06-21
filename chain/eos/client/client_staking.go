package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/eos/builder"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
)

func (client *Client) FetchStakeBalance(ctx context.Context, args xclient.StakedBalanceArgs) ([]*xclient.StakedBalance, error) {
	from := args.GetFrom()
	accounts, err := client.api.GetAccountsByAuthorizers(ctx, []eos.PermissionLevel{}, []string{string(from)})
	if err != nil {
		return nil, fmt.Errorf("failed to get EOS accounts: %w", err)
	}
	balances := []*xclient.StakedBalance{}
	for _, account := range accounts.Accounts {
		info, err := client.api.GetAccount(ctx, account.Account)
		if err != nil {
			return nil, fmt.Errorf("failed to get EOS account: %w", err)
		}

		cpuAmountStaked := xc.NewAmountBlockchainFromUint64(uint64(info.SelfDelegatedBandwidth.CPUWeight.Amount))
		netAmountStaked := xc.NewAmountBlockchainFromUint64(uint64(info.SelfDelegatedBandwidth.NetWeight.Amount))

		cpuAmountUnstaking := xc.NewAmountBlockchainFromUint64(uint64(info.RefundRequest.CPUAmount.Amount))
		netAmountUnstaking := xc.NewAmountBlockchainFromUint64(uint64(info.RefundRequest.NetAmount.Amount))

		if !cpuAmountStaked.IsZero() {
			balances = append(balances, xclient.NewStakedBalance(cpuAmountStaked, xclient.Active, string(builder.CPU), string(account.Account)))
		}
		if !netAmountStaked.IsZero() {
			balances = append(balances, xclient.NewStakedBalance(netAmountStaked, xclient.Active, string(builder.NET), string(account.Account)))
		}
		if !cpuAmountUnstaking.IsZero() {
			balances = append(balances, xclient.NewStakedBalance(cpuAmountUnstaking, xclient.Deactivating, string(builder.CPU), string(account.Account)))
		}
		if !netAmountUnstaking.IsZero() {
			balances = append(balances, xclient.NewStakedBalance(netAmountUnstaking, xclient.Deactivating, string(builder.NET), string(account.Account)))
		}
	}

	return balances, nil
}

func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	from := args.GetFrom()
	account, _ := args.GetFromIdentity()
	input, err := client.FetchBaseTxInput(ctx, AddressAndAccount{Address: from, Account: account}, AddressAndAccount{}, "")
	if err != nil {
		return nil, err
	}
	return &tx_input.StakingInput{
		TxInput: *input,
	}, nil
}

func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	from := args.GetFrom()
	account, _ := args.GetFromIdentity()
	input, err := client.FetchBaseTxInput(ctx, AddressAndAccount{Address: from, Account: account}, AddressAndAccount{}, "")
	if err != nil {
		return nil, err
	}
	return &tx_input.UnstakingInput{
		TxInput: *input,
	}, nil
}

func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	from := args.GetFrom()
	account, _ := args.GetFromIdentity()
	input, err := client.FetchBaseTxInput(ctx, AddressAndAccount{Address: from, Account: account}, AddressAndAccount{}, "")
	if err != nil {
		return nil, err
	}
	return &tx_input.WithdrawInput{
		TxInput: *input,
	}, nil
}
