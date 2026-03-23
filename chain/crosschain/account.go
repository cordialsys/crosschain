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

	apiURL := fmt.Sprintf("%s/v1/chains/%s/accounts", client.URL, chain)
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
	if err != nil {
		return nil, err
	}

	r := types.TxInputRes{}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, err
	}
	if r.TxInput == "" {
		return nil, nil
	}
	return drivers.UnmarshalCreateAccountInput([]byte(r.TxInput))
}

func (client *Client) GetAccountState(ctx context.Context, args *xclient.CreateAccountArgs) (*xclient.AccountState, error) {
	chain := client.Asset.GetChain().Chain
	req := &types.CreateAccountInputReq{
		Address:   string(args.GetAddress()),
		PublicKey: args.GetPublicKey(),
	}

	apiURL := fmt.Sprintf("%s/v1/chains/%s/accounts/state", client.URL, chain)
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
	if err != nil {
		return nil, err
	}

	r := types.AccountStateRes{}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, err
	}
	return r.AccountState, nil
}
