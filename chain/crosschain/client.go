package crosschain

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
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
var _ xclient.MultiTransferClient = &Client{}

const ServiceApiKeyHeader = "x-service-api-key"

// NewClient returns a new Crosschain Client
func NewClient(cfgI xc.ITask, url string, apiKeyRef config.Secret, network xc.NetworkSelector, httpTimeout time.Duration) (*Client, error) {
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
		Asset: cfgI,
		URL:   url,
		Http: &http.Client{
			// Prevent requests from hanging indefinitely
			Timeout: httpTimeout,
		},
		Network: network,
		ApiKey:  apiKey,
	}, nil
}

func NewStakingClient(cfgI xc.ITask, url string, apiKeyRef config.Secret, serviceApiKey config.Secret, provider xc.StakingProvider, network xc.NetworkSelector, httpTimeout time.Duration) (*Client, error) {
	client, err := NewClient(cfgI, url, apiKeyRef, network, httpTimeout)
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
			return nil, fmt.Errorf("(status=%d) %s", r.Code, string(bz))
		}
		status, ok := errors.FromGrpcCode(codes.Code(r.Code))
		if ok {
			// map to native error
			return nil, errors.Errorf(status, "%v", r.Message)
		}
		return nil, &r
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

	publicKeyMaybe, _ := args.GetPublicKey()
	memoMaybe, _ := args.GetMemo()
	priorityMaybe, _ := args.GetPriority()
	fromIdentityMaybe, _ := args.GetFromIdentity()
	feePayerIdentityMaybe, _ := args.GetFeePayerIdentity()

	res, err := client.legacyApiCall(ctx, "/input", &types.TransferInputReq{
		Chain:     client.Asset.GetChain().Chain,
		Contract:  string(contract),
		Balance:   args.GetAmount().String(),
		Decimals:  decimalsStr,
		PublicKey: hex.EncodeToString(publicKeyMaybe),
		From:      string(args.GetFrom()),
		To:        string(args.GetTo()),
		FeePayer:  types.NewFeePayerInfoOrNil(&args),
		Extra: types.TransferInputReqExtra{
			FromIdentity:        fromIdentityMaybe,
			FeePayerIdentity:    feePayerIdentityMaybe,
			TransactionAttempts: args.GetTransactionAttempts(),
			Memo:                memoMaybe,
			Priority:            string(priorityMaybe),
		},
	})
	if err != nil {
		return nil, err
	}

	var r = &types.LegacyTxInputRes{}
	_ = json.Unmarshal(res, r)
	return drivers.UnmarshalTxInput(r.NewTxInput)
}

func (client *Client) FetchMultiTransferInput(ctx context.Context, args xcbuilder.MultiTransferArgs) (xc.MultiTransferInput, error) {
	chain := client.Asset.GetChain().Chain
	apiURL := fmt.Sprintf("%s/v1/chains/%s/batch-transfers", client.URL, chain)

	memoMaybe, _ := args.GetMemo()
	priorityMaybe, _ := args.GetPriority()
	senders := []*types.Sender{}
	receivers := []*types.Receiver{}
	for _, sender := range args.Spenders() {
		fromIdentityMaybe, _ := sender.GetFromIdentity()
		senders = append(senders, &types.Sender{
			Address:   sender.GetFrom(),
			PublicKey: hex.EncodeToString(sender.GetPublicKey()),
			Extra:     types.SenderExtra{Identity: fromIdentityMaybe},
		})
	}
	for _, receiver := range args.Receivers() {
		toMemoMaybe, _ := receiver.GetMemo()
		contract, _ := receiver.GetContract()
		decimals, _ := receiver.GetDecimals()
		receivers = append(receivers, &types.Receiver{
			Address:  receiver.GetTo(),
			Balance:  receiver.GetAmount(),
			Memo:     toMemoMaybe,
			Contract: contract,
			Decimals: decimals,
		})
	}
	res, err := client.ApiCallWithUrl(ctx, "POST", apiURL, &types.MultiTransferInputReq{
		Chain:     client.Asset.GetChain().Chain,
		Senders:   senders,
		Receivers: receivers,
		FeePayer:  types.NewFeePayerInfoOrNil(&args),
		Extra: types.MultiTransferInputReqExtra{
			Priority:            string(priorityMaybe),
			Memo:                memoMaybe,
			TransactionAttempts: args.GetTransactionAttempts(),
		},
	})
	if err != nil {
		return nil, err
	}
	var r = &types.TransferInputRes{}
	_ = json.Unmarshal(res, r)
	return drivers.UnmarshalMultiTransferInput([]byte(r.Input))
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
	var metadataBz []byte
	if txWithMetadata, ok := txInput.(xc.TxWithMetadata); ok {
		bz, err := txWithMetadata.GetMetadata()
		if err != nil {
			return err
		}
		metadataBz = bz
	}

	var signatures [][]byte
	if txLegacyGetSignatures, ok := txInput.(xc.TxLegacyGetSignatures); ok {
		for _, sig := range txLegacyGetSignatures.GetSignatures() {
			signatures = append(signatures, sig)
		}
	}
	req := &types.SubmitTxReq{
		Chain:              chain,
		TxData:             data,
		LegacyTxSignatures: signatures,
		BroadcastInput:     string(metadataBz),
	}
	res, err := client.legacyApiCall(ctx, "/submit", req)
	if err != nil {
		return err
	}

	var r types.SubmitTxRes
	err = json.Unmarshal(res, &r)
	return err
}

// FetchLegacyTxInfo returns tx info from a Crosschain endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	res, err := client.legacyApiCall(ctx, "/info", &types.TxInfoReq{
		Chain:  client.Asset.GetChain().Chain,
		TxHash: string(txHash),
	})
	if err != nil {
		return xclient.LegacyTxInfo{}, err
	}

	var r types.TxLegacyInfoRes
	err = json.Unmarshal(res, &r)
	return r.LegacyTxInfo, err
}

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	txHashStr := args.TxHash()
	chain := client.Asset.GetChain().Chain

	query := url.Values{}
	if contract, ok := args.Contract(); ok {
		query.Add("contract", string(contract))
	}
	if sender, ok := args.Sender(); ok {
		query.Add("sender", string(sender))
	}
	if txTime, ok := args.TxSignTime(); ok {
		ts := time.Unix(txTime, 0).UTC().Format(time.RFC3339)
		query.Add("sign_time", ts)
	}
	if blockHeight, ok := args.BlockHeight(); ok {
		query.Add("block.height", strconv.FormatUint(blockHeight, 10))
	}

	apiURL := fmt.Sprintf("%s/v1/chains/%s/transactions/%s?%s", client.URL, chain, txHashStr, query.Encode())
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
