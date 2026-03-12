package crosschain

import (
	"context"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers"
)

func (client *Client) FetchCreateAccountInput(ctx context.Context, args *xclient.CreateAccountArgs) (xc.CreateAccountTxInput, error) {
	chain := client.Asset.GetChain().Chain
	req := &types.CreateAccountInputReq{
		Address:   string(args.GetAddress()),
		PublicKey: args.GetPublicKey(),
	}

	apiURL := fmt.Sprintf("%s/v1/chains/%s/addresses/%s/register", client.URL, chain, args.GetAddress())
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
	if err != nil {
		return nil, err
	}

	r := types.TxInputRes{}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, err
	}
	if r.TxInput == "" {
		return nil, fmt.Errorf("server returned empty input")
	}
	return drivers.UnmarshalCreateAccountInput([]byte(r.TxInput))
}

func (client *Client) GetAccountState(ctx context.Context, args *xclient.CreateAccountArgs) (xclient.AccountState, error) {
	chain := client.Asset.GetChain().Chain

	apiURL := fmt.Sprintf("%s/v1/chains/%s/addresses/%s/state", client.URL, chain, args.GetAddress())
	res, err := client.ApiCallWithUrl(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	r := types.AccountStateRes{}
	if err := json.Unmarshal(res, &r); err != nil {
		return "", err
	}
	return r.State, nil
}
