package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/template/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

// Client for XRP
type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
}

var _ xclient.FullClient = &Client{}

// NewClient returns a new JSON-RPC Client to the XRP node
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()

	return &Client{
		Url:        cfg.URL,
		HttpClient: http.DefaultClient,
		Asset:      cfgI,
	}, nil
}

type RequestType string

const AccountInfo RequestType = "account_info"
const AccountLines RequestType = "account_lines"
const Transaction RequestType = "tx"
const Ledger RequestType = "ledger"

type Request struct {
	RequestType         RequestType
	AccountInfoRequest  *AccountInfoRequest
	AccountLinesRequest *AccountLinesRequest
	TransactionRequest  *TransactionRequest
	LedgerRequest       *LedgerRequest
}

type AccountInfoRequest struct {
	Method string                  `json:"method"`
	Params []AccountInfoParamEntry `json:"params"`
}

type AccountInfoParamEntry struct {
	Account xc.Address `json:"account"`
	//Strict      bool       `json:"strict"`
	//LedgerIndex string     `json:"ledger_index"`
	//Queue       bool       `json:"queue"`
}

type AccountLinesRequest struct {
	Method string                   `json:"method"`
	Params []AccountLinesParamEntry `json:"params"`
}

type AccountLinesParamEntry struct {
	Account xc.Address `json:"account"`
}

type TransactionRequest struct {
	Method string                  `json:"method"`
	Params []TransactionParamEntry `json:"params"`
}

type TransactionParamEntry struct {
	Transaction xc.TxHash `json:"transaction"`
	Binary      bool      `json:"binary"`
}

type LedgerRequest struct {
	Method string             `json:"method"`
	Params []LedgerParamEntry `json:"params"`
}

type LedgerParamEntry struct {
	LedgerIndex  string `json:"ledger_index"`
	Transactions bool   `json:"transactions"`
	Expand       bool   `json:"expand"`
	OwnerFunds   bool   `json:"owner_funds"`
}

type Response struct {
	AccountInfoResponse  *AccountInfoResponse
	AccountLinesResponse *AccountLinesResponse
	TransactionResponse  *TransactionResponse
	LedgerResponse       *LedgerResponse
}

type LedgerResponse struct {
	Result LedgerResult `json:"result"`
}

type LedgerResult struct {
	Ledger             LedgerInfo `json:"ledger"`
	LedgerCurrentIndex int64      `json:"ledger_current_index"`
	Validated          bool       `json:"validated"`
	Status             string     `json:"status"`
}

type LedgerInfo struct {
	Closed      bool   `json:"closed"`
	LedgerIndex string `json:"ledger_index"`
	ParentHash  string `json:"parent_hash"`
}

type TransactionResponse struct {
	Result TransactionResult `json:"result"`
}

type TransactionResult struct {
	Account            string          `json:"Account"`
	Amount             string          `json:"Amount"`
	Destination        string          `json:"Destination"`
	Fee                string          `json:"Fee"`
	Flags              int64           `json:"Flags"`
	LastLedgerSequence int64           `json:"LastLedgerSequence"`
	Sequence           int64           `json:"Sequence"`
	SigningPubKey      string          `json:"SigningPubKey"`
	TransactionType    string          `json:"TransactionType"`
	TxnSignature       string          `json:"TxnSignature"`
	Hash               string          `json:"hash"`
	DeliverMax         string          `json:"DeliverMax"`
	CtID               string          `json:"ctid"`
	Meta               TransactionMeta `json:"meta"`
	Validated          bool            `json:"validated"`
	Date               int64           `json:"date"`
	LedgerIndex        int64           `json:"ledger_index"`
	InLedger           int64           `json:"inLedger"`
	Status             string          `json:"status"`
}

type TransactionMeta struct {
	AffectedNodes     []AffectedNodes `json:"AffectedNodes"`
	TransactionIndex  int64           `json:"TransactionIndex"`
	TransactionResult string          `json:"TransactionResult"`
	DeliveredAmount   string          `json:"delivered_amount"`
}

type AffectedNodes struct {
	ModifiedNode ModifiedNode `json:"ModifiedNode"`
}

type ModifiedNode struct {
	FinalFields       FinalFields    `json:"FinalFields"`
	LedgerEntryType   string         `json:"LedgerEntryType"`
	LedgerIndex       string         `json:"LedgerIndex"`
	PreviousFields    PreviousFields `json:"PreviousFields"`
	PreviousTxnID     string         `json:"PreviousTxnID"`
	PreviousTxnLgrSeq int64          `json:"PreviousTxnLgrSeq"`
}

type FinalFields struct {
	Account    string `json:"Account"`
	Balance    string `json:"Balance"`
	Flags      int64  `json:"Flags"`
	OwnerCount int    `json:"OwnerCount"`
	Sequence   int64  `json:"Sequence"`
}

type PreviousFields struct {
	Balance string `json:"Balance"`
}

type AccountLinesResponse struct {
	Result AccountLinesResultDetails `json:"result"`
}

type AccountLinesResultDetails struct {
	LedgerHash  string `json:"LedgerHash"`
	LedgerIndex uint   `json:"LedgerIndex"`
	Validated   bool   `json:"Validated"`
	Status      string `json:"Status"`
	Lines       []Line `json:"lines"`
}

type Line struct {
	Account      string `json:"Account"`
	Balance      string `json:"balance"`
	Currency     string `json:"currency"`
	Limit        string `json:"limit"`
	LimitPeer    string `json:"limit_peer"`
	QualityIn    uint   `json:"quality_in"`
	QualityOut   uint   `json:"quality_out"`
	NoRipple     bool   `json:"no_ripple"`
	NoRipplePeer bool   `json:"no_ripple_peer"`
}

type AccountInfoResponse struct {
	Result AccountInfoResultDetails `json:"result"`
}

type AccountInfoResultDetails struct {
	AccountData AccountData `json:"account_data"`
}

type AccountData struct {
	Account           string `json:"Account"`
	Balance           string `json:"Balance"`
	Flags             uint64 `json:"Flags"`
	LedgerEntryType   string `json:"LedgerEntryType"`
	OwnerCount        uint   `json:"OwnerCount"`
	PreviousTxnID     string `json:"PreviousTxnID"`
	PreviousTxnLgrSeq uint   `json:"PreviousTxnLgrSeq"`
	Sequence          uint   `json:"Sequence"`
	Index             string `json:"Index"`
}

// FetchTransferInput returns tx input for a Template tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	return &tx_input.TxInput{}, errors.New("not implemented")
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	return errors.New("not implemented")
}

// FetchTxInfo Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	legacyTxInfo, err := client.FetchLegacyTxInfo(ctx, txHash)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// Remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain().Chain, legacyTxInfo, xclient.Account), nil
}

// FetchLegacyTxInfo Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {

	txRequest := Request{
		RequestType: Transaction,
		TransactionRequest: &TransactionRequest{
			Method: "tx",
			Params: []TransactionParamEntry{
				{
					Transaction: txHash,
					Binary:      false,
				},
			},
		},
	}

	response, err := client.ExecuteRequest(ctx, txRequest)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	txResponse := response.TransactionResponse

	ledgerRequest := Request{
		RequestType: Ledger,
		LedgerRequest: &LedgerRequest{
			Method: "ledger",
			Params: []LedgerParamEntry{
				{
					LedgerIndex: "current",
				},
			},
		},
	}

	response, err = client.ExecuteRequest(ctx, ledgerRequest)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	ledgerResponse := response.LedgerResponse

	confirmations := ledgerResponse.Result.LedgerCurrentIndex - txResponse.Result.Sequence

	explorer := client.Asset.GetChain().ExplorerURL + "/tx/" + txResponse.Result.Hash + "?cluster=" + client.Asset.GetChain().Net

	var sources []*xc.LegacyTxInfoEndpoint
	sources = append(sources, &xc.LegacyTxInfoEndpoint{
		Address: xc.Address(txResponse.Result.Account),
		Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.Amount),
	})

	var destinations []*xc.LegacyTxInfoEndpoint
	destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
		Address: xc.Address(txResponse.Result.Destination),
		Amount:  xc.NewAmountBlockchainFromStr(txResponse.Result.Amount),
	})

	var status xc.TxStatus
	if txResponse.Result.Status == "success" {
		status = xc.TxStatusSuccess
	} else if txResponse.Result.Status == "error" {
		status = xc.TxStatusFailure
	}

	txInfo := xc.LegacyTxInfo{
		BlockHash:     txResponse.Result.Hash,
		TxID:          txResponse.Result.Hash,
		ExplorerURL:   explorer,
		From:          xc.Address(txResponse.Result.Account),
		To:            xc.Address(txResponse.Result.Destination),
		Amount:        xc.NewAmountBlockchainFromStr(txResponse.Result.Amount),
		Fee:           xc.NewAmountBlockchainFromStr(txResponse.Result.Fee),
		BlockIndex:    txResponse.Result.LedgerIndex,
		BlockTime:     txResponse.Result.Date,
		Confirmations: confirmations,
		Status:        status,
		Sources:       sources,
		Destinations:  destinations,
		Time:          txResponse.Result.Date,
	}

	return txInfo, nil
}

// FetchBalance fetches token balance for a XRP address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalanceForAsset(ctx, address, client.Asset)
}

func (client *Client) FetchBalanceForAsset(ctx context.Context, address xc.Address, assetCfg xc.ITask) (xc.AmountBlockchain, error) {
	switch asset := assetCfg.(type) {
	case *xc.ChainConfig:
		return client.FetchNativeBalance(ctx, address)
	case *xc.TokenAssetConfig:
		return client.fetchContractBalance(ctx, address, asset.Contract)
	default:
		contract := asset.GetContract()
		logrus.WithFields(logrus.Fields{
			"chain":      asset.GetChain().Chain,
			"contract":   contract,
			"asset_type": fmt.Sprintf("%T", asset),
		}).Warn("fetching balance for unknown asset type")
		return client.fetchContractBalance(ctx, address, contract)
	}
}

// FetchNativeBalance fetches account native balance for a XRP address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	request := Request{
		RequestType: AccountInfo,
		AccountInfoRequest: &AccountInfoRequest{
			Method: "account_info",
			Params: []AccountInfoParamEntry{
				{
					Account: address,
				},
			},
		},
	}

	response, err := client.ExecuteRequest(ctx, request)
	if err != nil {
		return zero, err
	}
	accountInfoResponse := response.AccountInfoResponse

	balance := accountInfoResponse.Result.AccountData.Balance
	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	return xc.NewAmountBlockchainFromStr(balance), nil
}

// fetchContractBalance fetches a specific token balance based on received contract for an XRP address
func (client *Client) fetchContractBalance(ctx context.Context, address xc.Address, assetContract string) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	asset, contract, err := extractAssetAndContract(assetContract)
	if err != nil {
		return zero, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}

	request := Request{
		RequestType: AccountLines,
		AccountLinesRequest: &AccountLinesRequest{
			Method: "account_lines",
			Params: []AccountLinesParamEntry{
				{
					Account: address,
				},
			},
		},
	}

	response, err := client.ExecuteRequest(ctx, request)
	if err != nil {
		return zero, err
	}
	accountLinesResponse := response.AccountLinesResponse

	var balance string
	for _, line := range accountLinesResponse.Result.Lines {
		if line.Currency == asset && line.Account == contract {
			balance = line.Balance
		}
	}

	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	bBalance := xc.NewAmountBlockchainFromStr(balance)
	return bBalance, nil
}

func (client *Client) ExecuteRequest(ctx context.Context, request Request) (*Response, error) {
	var (
		requestPayload interface{}
		response       interface{}
	)

	switch request.RequestType {
	case AccountInfo:
		requestPayload = request.AccountInfoRequest
		response = &AccountInfoResponse{}
	case AccountLines:
		requestPayload = request.AccountLinesRequest
		response = &AccountLinesResponse{}
	case Transaction:
		requestPayload = request.TransactionRequest
		response = &TransactionResponse{}
	case Ledger:
		requestPayload = request.LedgerRequest
		response = &LedgerResponse{}
	}

	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", client.Url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch balance, HTTP status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	result := Response{}
	switch request.RequestType {
	case AccountInfo:
		result.AccountInfoResponse = response.(*AccountInfoResponse)
	case AccountLines:
		result.AccountLinesResponse = response.(*AccountLinesResponse)
	case Transaction:
		result.TransactionResponse = response.(*TransactionResponse)
	case Ledger:
		result.LedgerResponse = response.(*LedgerResponse)
	}

	return &result, nil
}

// extractAssetAndContract parse assetContract and returns asset and contract
func extractAssetAndContract(assetContract string) (string, string, error) {
	var separator string

	switch {
	case strings.Contains(assetContract, "."):
		separator = "."
	case strings.Contains(assetContract, "-"):
		separator = "-"
	case strings.Contains(assetContract, "_"):
		separator = "_"
	default:
		return "", "", fmt.Errorf("string must contain one of the following separators: '.', '-', '_'")
	}

	parts := strings.Split(assetContract, separator)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format, string should contain exactly one separator")
	}

	asset := parts[0]
	contract := parts[1]

	return asset, contract, nil
}
