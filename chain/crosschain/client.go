package crosschain

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/sirupsen/logrus"
)

// Client for Template
type Client struct {
	Asset           xc.ITask
	URL             string
	Http            *http.Client
	Network         xc.NetworkSelector
	StakingProvider xc.StakingProvider
	ApiKey          string
	ServiceApiKey   string
}

var _ xclient.FullClient = &Client{}
var _ xclient.StakingClient = &Client{}

const ServiceApiKeyHeader = "x-service-api-key"

// NewClient returns a new Crosschain Client
func NewClient(cfgI xc.ITask, apiKey string) (*Client, error) {
	url := cfgI.GetChain().Clients[0].URL
	url = strings.TrimSuffix(url, "/")
	network := cfgI.GetChain().Clients[0].Network

	if config.HasTypePrefix(apiKey) {
		var err error
		apiKey, err = config.GetSecret(apiKey)
		if err != nil {
			logrus.WithError(err).Warn("failed to get connector api key")
		}
	}
	return &Client{
		Asset:   cfgI,
		URL:     url,
		Http:    &http.Client{},
		Network: network,
		ApiKey:  apiKey,
	}, nil
}

func NewStakingClient(cfgI xc.ITask, apiKey string, serviceApiKey config.Secret, provider xc.StakingProvider) (*Client, error) {
	client, err := NewClient(cfgI, apiKey)
	if err != nil {
		return nil, err
	}
	client.ServiceApiKey, err = serviceApiKey.Load()
	if err != nil {
		logrus.WithError(err).WithField("service", provider).Warn("failed to get service api key")
	}
	client.StakingProvider = provider
	return client, nil
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

func (client *Client) legacyApiCall(ctx context.Context, path string, data interface{}) ([]byte, error) {
	// Create HTTP POST request
	apiURL := fmt.Sprintf("%s/v1/__crosschain%s", client.URL, path)
	response, err := client.ApiCallWithUrl(ctx, "POST", apiURL, data)
	if err != nil {
		return response, err
	}

	return response, nil
}

// Base64 encode if needed
func encodeApiKeyUserPassword(userPwMaybe string) string {
	if strings.Contains(userPwMaybe, ":") {
		return base64.StdEncoding.EncodeToString([]byte(userPwMaybe))
	}
	return userPwMaybe
}

func (client *Client) ApiCallWithUrl(ctx context.Context, method string, url string, data interface{}) ([]byte, error) {
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
	if client.Network != "" {
		req.Header.Add("network", string(client.Network))
	}
	if client.ApiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodeApiKeyUserPassword(client.ApiKey)))
	}
	if client.ServiceApiKey != "" {
		req.Header.Set(ServiceApiKeyHeader, client.ServiceApiKey)
	}
	logrus.WithFields(logrus.Fields{
		"method":  method,
		"url":     url,
		"network": client.Network,
	}).Debug("connector request")

	// Send the request
	res, err := client.Http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	bz, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"body":   string(bz),
		"status": res.StatusCode,
	}).Debug("connector response")

	// Return error if HTTP return error
	if res.StatusCode != 200 {
		var r types.Status
		err = json.Unmarshal(bz, &r)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", r.Message)
	}

	return bz, nil
}

// FetchLegacyTxInput returns tx input from a Crosschain endpoint
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	res, err := client.legacyApiCall(ctx, "/input", &types.TxInputReq{
		AssetReq: client.apiAsset(),
		From:     string(args.GetFrom()),
		To:       string(args.GetTo()),
	})
	if err != nil {
		return nil, err
	}

	var r = &types.LegacyTxInputRes{}
	_ = json.Unmarshal(res, r)
	return drivers.UnmarshalTxInput(r.TxInput)
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
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

	res, err := client.legacyApiCall(ctx, "/submit", &types.SubmitTxReq{
		ChainReq:     &types.ChainReq{Chain: chain},
		TxData:       data,
		TxSignatures: signatures,
	})
	if err != nil {
		return err
	}

	var r types.SubmitTxRes
	err = json.Unmarshal(res, &r)
	return err
}

// FetchLegacyTxInfo returns tx info from a Crosschain endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	res, err := client.legacyApiCall(ctx, "/info", &types.TxInfoReq{
		AssetReq: client.apiAsset(),
		TxHash:   string(txHash),
	})
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	var r types.TxLegacyInfoRes
	err = json.Unmarshal(res, &r)
	return r.LegacyTxInfo, err
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	chain := client.Asset.GetChain().Chain
	apiURL := fmt.Sprintf("%s/v1/chains/%s/transactions/%s", client.URL, chain, txHashStr)
	res, err := client.ApiCallWithUrl(ctx, "GET", apiURL, nil)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	r := types.TransactionInfoRes{}
	err = json.Unmarshal(res, &r)
	return r.TxInfo, err
}

// FetchNativeBalance fetches account balance from a Crosschain endpoint
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	var assetReq = client.apiAsset()
	assetReq.Asset = ""
	assetReq.Contract = ""
	assetReq.Decimals = ""
	res, err := client.legacyApiCall(ctx, "/balance", &types.BalanceReq{
		AssetReq: assetReq,
		Address:  string(address),
	})
	if err != nil {
		return zero, err
	}

	var r types.BalanceRes
	err = json.Unmarshal(res, &r)
	return r.BalanceRaw, err
}

// FetchBalance fetches token balance from a Crosschain endpoint
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	res, err := client.legacyApiCall(ctx, "/balance", &types.BalanceReq{
		AssetReq: client.apiAsset(),
		Address:  string(address),
	})
	if err != nil {
		return zero, err
	}

	var r types.BalanceRes
	err = json.Unmarshal(res, &r)
	return r.BalanceRaw, err
}
