package crosschain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers"
)

// Client for Template
type Client struct {
	Asset xc.ITask
	URL   string
	Http  *http.Client
}

var _ xclient.FullClient = &Client{}

// TxInput for Template
type TxInput struct {
}

// NewClient returns a new Crosschain Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	url := cfgI.GetChain().Clients[0].URL
	return &Client{
		Asset: cfgI,
		URL:   url,
		Http:  &http.Client{},
	}, nil
}

func (client *Client) nextClient() (xclient.FullClient, error) {
	cfg := client.Asset
	driver := cfg.GetChain().Driver
	if driver == "" {
		return nil, errors.New("crosschain client fallback is disabled")
	}
	return drivers.NewClient(cfg, xc.Driver(driver))
}

func (client *Client) apiAsset() *types.AssetReq {
	native := client.Asset.GetChain()
	contract := client.Asset.GetContract()
	decimals := client.Asset.GetDecimals()
	assetSymbol := client.Asset.GetAssetSymbol()

	return &types.AssetReq{
		ChainReq: &types.ChainReq{Chain: string(native.Chain)},
		Asset:    assetSymbol,
		Contract: contract,
		Decimals: strconv.FormatInt(int64(decimals), 10),
	}
}

func (client *Client) apiCall(ctx context.Context, path string, data interface{}) ([]byte, error) {
	// Create HTTP POST request
	apiURL := fmt.Sprintf("%s/v1/__crosschain%s", client.URL, path)
	response, err := client.apiCallWithUrl(ctx, "POST", apiURL, data)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (client *Client) apiCallWithUrl(ctx context.Context, method string, url string, data interface{}) ([]byte, error) {
	// Serialize the request
	var req *http.Request
	var err error
	if data != nil {
		buf := new(bytes.Buffer)
		json.NewEncoder(buf).Encode(data)
		req, err = http.NewRequestWithContext(ctx, method, url, buf)
	} else {
		// provide untyped nil to use no body. any "typed" nil will cause panic.
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	// Send the request
	res, err := client.Http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Return error if HTTP return error
	if res.StatusCode != 200 {
		var r types.Status
		err = json.NewDecoder(res.Body).Decode(&r)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%d: %s", r.Code, r.Message)
	}

	bz, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

// FetchTxInput returns tx input from a Crosschain endpoint
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	res, err := client.apiCall(ctx, "/input", &types.TxInputReq{
		AssetReq: client.apiAsset(),
		From:     string(from),
		To:       string(to),
	})
	if err != nil {
		return nil, err
	}
	// if err != nil {
	// 	// Fallback to default client
	// 	nextClient, err2 := client.nextClient()
	// 	if err2 != nil {
	// 		return nil, err
	// 	}
	// 	log.Printf("crosschain client.FetchTxInput - fall back to node err=%s", err)
	// 	return nextClient.FetchTxInput(ctx, from, to)
	// }
	var r = &types.TxInputRes{}
	_ = json.Unmarshal(res, r)
	return drivers.UnmarshalTxInput(r.TxInput)
}

// SubmitTx submits via a Crosschain endpoint
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	chain := string(client.Asset.GetChain().Chain)
	data, err := txInput.Serialize()
	if err != nil {
		return err
	}
	xcSignatures := txInput.GetSignatures()
	signatures := [][]byte{}
	for _, sig := range xcSignatures {
		signatures = append(signatures, sig)
	}

	res, err := client.apiCall(ctx, "/submit", &types.SubmitTxReq{
		ChainReq:     &types.ChainReq{Chain: chain},
		TxData:       data,
		TxSignatures: signatures,
	})
	if err != nil {
		return err
	}
	// if err != nil {
	// 	// Fallback to default client
	// 	nextClient, err2 := client.nextClient()
	// 	if err2 != nil {
	// 		return err
	// 	}
	// 	log.Printf("crosschain client.SubmitTx - fall back to node err=%s", err)
	// 	return nextClient.SubmitTx(ctx, txInput)
	// }
	var r types.SubmitTxRes
	err = json.Unmarshal(res, &r)
	return err
}

// FetchLegacyTxInfo returns tx info from a Crosschain endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	res, err := client.apiCall(ctx, "/info", &types.TxInfoReq{
		AssetReq: client.apiAsset(),
		TxHash:   string(txHash),
	})
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	// if err != nil {
	// 	// Fallback to default client
	// 	nextClient, err2 := client.nextClient()
	// 	if err2 != nil {
	// 		return xc.LegacyTxInfo{}, err
	// 	}
	// 	log.Printf("crosschain client.FetchLegacyTxInfo - fall back to node err=%s", err)
	// 	return nextClient.FetchLegacyTxInfo(ctx, txHash)
	// }
	var r types.TxLegacyInfoRes
	err = json.Unmarshal(res, &r)
	return r.LegacyTxInfo, err
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	chain := client.Asset.GetChain().Chain
	apiURL := fmt.Sprintf("%s/v1/chains/%s/transactions/%s", client.URL, chain, txHashStr)
	res, err := client.apiCallWithUrl(ctx, "GET", apiURL, nil)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	// if err != nil {
	// 	// Fallback to default client
	// 	nextClient, err2 := client.nextClient()
	// 	if err2 != nil {
	// 		return xclient.TxInfo{}, err
	// 	}
	// 	log.Printf("crosschain client.FetchLegacyTxInfo - fall back to node err=%s", err)
	// 	return nextClient.FetchTxInfo(ctx, txHashStr)
	// }
	r := types.TransactionInfoRes{}
	err = json.Unmarshal(res, &r)
	return r.TxInfo, err
}

// FetchNativeBalance fetches account balance from a Crosschain endpoint
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	// res, err := client.apiCall(ctx, "/balance/native", &types.BalanceReq{
	// 	AssetReq: client.apiAsset(),
	// 	Address:  string(address),
	// })
	var assetReq = client.apiAsset()
	assetReq.Asset = ""
	assetReq.Contract = ""
	assetReq.Decimals = ""
	res, err := client.apiCall(ctx, "/balance", &types.BalanceReq{
		AssetReq: assetReq,
		Address:  string(address),
	})
	if err != nil {
		return zero, err
	}
	// if err != nil {
	// 	// Fallback to default client
	// 	nextClient, err2 := client.nextClient()
	// 	if err2 != nil {
	// 		return zero, err
	// 	}
	// 	log.Printf("crosschain client.FetchNativeBalance - fall back to node err=%s", err)
	// 	return nextClient.FetchNativeBalance(ctx, address)
	// }
	var r types.BalanceRes
	err = json.Unmarshal(res, &r)
	return r.BalanceRaw, err
}

// FetchBalance fetches token balance from a Crosschain endpoint
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	res, err := client.apiCall(ctx, "/balance", &types.BalanceReq{
		AssetReq: client.apiAsset(),
		Address:  string(address),
	})
	if err != nil {
		return zero, err
	}
	// if err != nil {
	// 	// Fallback to default client
	// 	nextClient, err2 := client.nextClient()
	// 	if err2 != nil {
	// 		return zero, err
	// 	}
	// 	log.Printf("crosschain client.FetchBalance - fall back to node err=%s", err)
	// 	return nextClient.FetchNativeBalance(ctx, address)
	// }
	var r types.BalanceRes
	err = json.Unmarshal(res, &r)
	return r.BalanceRaw, err
}
