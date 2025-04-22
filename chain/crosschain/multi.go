package crosschain

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"

// 	xc "github.com/cordialsys/crosschain"
// 	xcbuilder "github.com/cordialsys/crosschain/builder"
// 	"github.com/cordialsys/crosschain/chain/crosschain/types"
// 	"github.com/cordialsys/crosschain/factory/drivers"
// )

// func (client *Client) FetchMultiTransferInput(ctx context.Context, args xcbuilder.MultiTransferArgs) (xc.MultiTransferInput, error) {
// 	chain := client.Asset.GetChain().Chain

// 	var req = &types.MultiTransferInputReq{
// 		From:     string(args.GetFrom()),
// 		Balance:  args.GetAmount().String(),
// 		Provider: client.StakingProvider,
// 		FeePayer: types.NewFeePayerInfoOrNil(&args),
// 	}

// 	apiURL := fmt.Sprintf("%s/v1/chains/%s/stakes", client.URL, chain)
// 	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	r := types.TxInputRes{}
// 	err = json.Unmarshal(res, &r)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return drivers.UnmarshalStakingInput([]byte(r.TxInput))
// }
