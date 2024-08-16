package crosschain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers"
)

func (client *Client) FetchStakeBalance(ctx context.Context, args xcclient.StakedBalanceArgs) ([]*xcclient.StakedBalance, error) {
	chain := client.Asset.GetChain().Chain

	params := url.Values{}
	if validator, ok := args.GetValidator(); ok {
		params.Set("validator", validator)
	}
	if account, ok := args.GetAccount(); ok {
		params.Set("account", account)
	}
	if client.StakingProvider != "" {
		params.Set("provider", string(client.StakingProvider))
	}

	apiURL := fmt.Sprintf("%s/v1/chains/%s/addresses/%s/staking?%s", client.URL, chain, args.GetFrom(), params.Encode())
	res, err := client.ApiCallWithUrl(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	r := []*xcclient.StakedBalance{}
	err = json.Unmarshal(res, &r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (client *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	chain := client.Asset.GetChain().Chain

	var req = &types.StakingInputReq{
		From:     string(args.GetFrom()),
		Balance:  args.GetAmount().String(),
		Provider: client.StakingProvider,
	}
	req.Validator, _ = args.GetValidator()
	req.Account, _ = args.GetStakeAccount()

	apiURL := fmt.Sprintf("%s/v1/chains/%s/stakes", client.URL, chain)
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
	if err != nil {
		return nil, err
	}

	r := types.TxInputRes{}
	err = json.Unmarshal(res, &r)
	if err != nil {
		return nil, err
	}
	return drivers.UnmarshalStakingInput([]byte(r.TxInput))
}
func (client *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	chain := client.Asset.GetChain().Chain

	var req = &types.StakingInputReq{
		From:     string(args.GetFrom()),
		Balance:  args.GetAmount().String(),
		Provider: client.StakingProvider,
	}
	req.Validator, _ = args.GetValidator()
	req.Account, _ = args.GetStakeAccount()

	apiURL := fmt.Sprintf("%s/v1/chains/%s/unstakes", client.URL, chain)
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
	if err != nil {
		return nil, err
	}

	r := types.TxInputRes{}
	err = json.Unmarshal(res, &r)
	if err != nil {
		return nil, err
	}
	return drivers.UnmarshalUnstakingInput([]byte(r.TxInput))
}
func (client *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	chain := client.Asset.GetChain().Chain

	var req = &types.StakingInputReq{
		From:     string(args.GetFrom()),
		Balance:  args.GetAmount().String(),
		Provider: client.StakingProvider,
	}
	req.Validator, _ = args.GetValidator()
	req.Account, _ = args.GetStakeAccount()

	apiURL := fmt.Sprintf("%s/v1/chains/%s/withdraws", client.URL, chain)
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
	if err != nil {
		return nil, err
	}

	r := types.TxInputRes{}
	err = json.Unmarshal(res, &r)
	if err != nil {
		return nil, err
	}
	return drivers.UnmarshalWithdrawingInput([]byte(r.TxInput))
}
