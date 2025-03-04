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

var _ xclient.Client = &Client{}
var _ xclient.StakingClient = &Client{}

const ServiceApiKeyHeader = "x-service-api-key"

// NewClient returns a new Crosschain Client
func NewClient(cfgI xc.ITask, url string, apiKeyRef config.Secret, network xc.NetworkSelector) (*Client, error) {
	url = strings.TrimSuffix(url, "/")
	var apiKey string
	var err error
	if apiKeyRef != "" {
		apiKey, err = apiKeyRef.Load()
		if err != nil {
			return nil, fmt.Errorf("could not load api-key: %v", err)
		}
	}

	if apiKey == "" {
		logrus.WithError(err).Warn("connector api key is empty")
	}

	return &Client{
		Asset:   cfgI,
		URL:     url,
		Http:    &http.Client{},
		Network: network,
		ApiKey:  apiKey,
	}, nil
}

func NewStakingClient(cfgI xc.ITask, url string, apiKeyRef config.Secret, serviceApiKey config.Secret, provider xc.StakingProvider, network xc.NetworkSelector) (*Client, error) {
	client, err := NewClient(cfgI, url, apiKeyRef, network)
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

func asJson(data interface{}) string {
	if data != nil {
		bz, _ := json.MarshalIndent(data, "", "  ")
		return string(bz)
	} else {
		return ""
	}
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
		"body":    asJson(data),
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
	contract, _ := args.GetContract()
	decimalsStr := ""
	if decimals, ok := args.GetDecimals(); ok {
		decimalsStr = strconv.FormatInt(int64(decimals), 10)
	}

	res, err := client.legacyApiCall(ctx, "/input", &types.TxInputReq{
		Chain:    client.Asset.GetChain().Chain,
		Contract: string(contract),
		Balance:  args.GetAmount().String(),
		Decimals: decimalsStr,
		From:     string(args.GetFrom()),
		To:       string(args.GetTo()),
	})
	if err != nil {
		return nil, err
	}

	var r = &types.LegacyTxInputRes{}
	_ = json.Unmarshal(res, r)
	return drivers.UnmarshalTxInput(r.NewTxInput)
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits via a Crosschain endpoint
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	chain := client.Asset.GetChain().Chain
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
		Chain:        chain,
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
		Chain:  client.Asset.GetChain().Chain,
		TxHash: string(txHash),
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

	res, err := client.legacyApiCall(ctx, "/balance", &types.BalanceReq{
		Chain:    client.Asset.GetChain().Chain,
		Contract: "",
		Address:  string(address),
	})
	if err != nil {
		return zero, err
	}

	var r types.BalanceRes
	err = json.Unmarshal(res, &r)

	return r.GetBalance(), err
}

// FetchBalance fetches token balance from a Crosschain endpoint
func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	contract, _ := args.Contract()
	res, err := client.legacyApiCall(ctx, "/balance", &types.BalanceReq{
		Chain:    client.Asset.GetChain().Chain,
		Contract: string(contract),
		Address:  string(args.Address()),
	})
	if err != nil {
		return zero, err
	}

	var r types.BalanceRes
	err = json.Unmarshal(res, &r)
	return r.GetBalance(), err
}

// FetchBalance fetches token balance from a Crosschain endpoint
func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	apiURL := fmt.Sprintf("%s/v1/chains/%s/assets/%s/decimals", client.URL, client.Asset.GetChain().Chain, contract)
	res, err := client.ApiCallWithUrl(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, err
	}

	asString := string(res)
	asString = strings.Trim(asString, "\"")
	dec, err := strconv.Atoi(asString)

	return dec, err
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	apiURL := fmt.Sprintf("%s/v1/chains/%s/block", client.URL, client.Asset.GetChain().Chain)
	height, ok := args.Height()
	if ok {
		apiURL = fmt.Sprintf("%s/v1/chains/%s/blocks/%d", client.URL, client.Asset.GetChain().Chain, height)
	}

	res, err := client.ApiCallWithUrl(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	var apiBlock types.BlockResponse
	err = json.Unmarshal(res, &apiBlock)
	if err != nil {
		return nil, err
	}
	block, err := types.UnpackBlock(&apiBlock)
	if err != nil {
		return nil, err
	}
	block.SubBlocks, err = types.UnpackSubBlocks(apiBlock.SubBlocks)
	if err != nil {
		return nil, err
	}

	return block, err
}
