package crosschain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	xc "github.com/jumpcrypto/crosschain"
	"github.com/jumpcrypto/crosschain/chain/crosschain/types"
	"github.com/jumpcrypto/crosschain/factory/drivers"
)

// Client for Template
type Client struct {
	Asset xc.ITask
	URL   string
	Http  *http.Client
}

// TxInput for Template
type TxInput struct {
}

// NewClient returns a new Crosschain Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	url := cfgI.GetNativeAsset().Clients[0].URL
	return &Client{
		Asset: cfgI,
		URL:   url,
		Http:  &http.Client{},
	}, nil
}

func (client *Client) nextClient() (xc.Client, error) {
	cfg := client.Asset
	driver := xc.Driver(cfg.GetNativeAsset().Driver)
	return drivers.NewClient(cfg, driver)
}

func (client *Client) apiAsset() *types.AssetReq {
	assetCfg := client.Asset.GetAssetConfig()
	return &types.AssetReq{
		Chain:    string(assetCfg.NativeAsset),
		Asset:    assetCfg.Asset,
		Contract: assetCfg.Contract,
		Decimals: assetCfg.Decimals,
	}
}

func (client *Client) apiCall(ctx context.Context, url string, data interface{}) ([]byte, error) {
	// Serialize the request
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(data)

	// Create HTTP POST request
	apiURL := fmt.Sprintf("%s/api/v1/crosschain%s", client.URL, url)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, buf)
	if err != nil {
		return nil, err
	}

	// Send the request
	res, err := client.Http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Parse API response
	var r types.ApiResponse
	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		return nil, err
	}

	// Return API error
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}

	// Return result
	// The result here is map[string]interface{}, in order to cast it
	// in the caller the easier way is to re-serialize it and let the
	// caller deserialize it.
	return json.Marshal(r.Result)
}

// FetchTxInput returns tx input from a Crosschain endpoint
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	res, err := client.apiCall(ctx, "/input", &types.TxInputReq{
		AssetReq: client.apiAsset(),
		From:     string(from),
		To:       string(to),
	})
	if err != nil {
		// Fallback to default client
		nextClient, err2 := client.nextClient()
		if err2 != nil {
			return nil, err
		}
		return nextClient.FetchTxInput(ctx, from, to)
	}
	var r types.TxInputRes
	_ = json.Unmarshal(res, &r)
	rSer, _ := json.Marshal(r.TxInput)
	return drivers.UnmarshalTxInput(rSer)
}

// SubmitTx submits via a Crosschain endpoint
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	chain := string(client.Asset.GetNativeAsset().NativeAsset)
	data, err := txInput.Serialize()
	if err != nil {
		return err
	}
	res, err := client.apiCall(ctx, "/submit", &types.SubmitTxReq{
		Chain:  chain,
		TxData: data,
	})
	if err != nil {
		// Fallback to default client
		nextClient, err2 := client.nextClient()
		if err2 != nil {
			return err
		}
		return nextClient.SubmitTx(ctx, txInput)
	}
	var r types.SubmitTxRes
	err = json.Unmarshal(res, &r)
	return err
}

// FetchTxInfo returns tx info from a Crosschain endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xc.TxInfo, error) {
	res, err := client.apiCall(ctx, "/input", &types.TxInfoReq{
		AssetReq: client.apiAsset(),
		TxHash:   string(txHash),
	})
	if err != nil {
		// Fallback to default client
		nextClient, err2 := client.nextClient()
		if err2 != nil {
			return xc.TxInfo{}, err
		}
		return nextClient.FetchTxInfo(ctx, txHash)
	}
	var r types.TxInfoRes
	err = json.Unmarshal(res, &r)
	return r.TxInfo, err
}

// FetchNativeBalance fetches account balance from a Crosschain endpoint
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	res, err := client.apiCall(ctx, "/balance/native", &types.BalanceReq{
		AssetReq: client.apiAsset(),
		Address:  string(address),
	})
	if err != nil {
		// Fallback to default client
		nextClient, err2 := client.nextClient()
		if err2 != nil {
			return zero, err
		}
		return nextClient.(xc.ClientBalance).FetchNativeBalance(ctx, address)
	}
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
		// Fallback to default client
		nextClient, err2 := client.nextClient()
		if err2 != nil {
			return zero, err
		}
		return nextClient.(xc.ClientBalance).FetchNativeBalance(ctx, address)
	}
	var r types.BalanceRes
	err = json.Unmarshal(res, &r)
	return r.BalanceRaw, err
}
