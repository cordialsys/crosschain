package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xrptx "github.com/cordialsys/crosschain/chain/xrp/tx"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
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

const MethodPost string = "POST"

const AccountInfo string = "account_info"
const AccountLines string = "account_lines"
const Transaction string = "tx"
const Ledger string = "ledger"
const Submit string = "submit"

type LedgerIndex string

const Validated LedgerIndex = "validated"
const Current LedgerIndex = "current"

type AccountInfoRequest struct {
	Method string                  `json:"method"`
	Params []AccountInfoParamEntry `json:"params"`
}

type AccountInfoParamEntry struct {
	Account     xc.Address  `json:"account"`
	LedgerIndex LedgerIndex `json:"ledger_index"`
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
	LedgerIndex  LedgerIndex `json:"ledger_index"`
	Transactions bool        `json:"transactions"`
	Expand       bool        `json:"expand"`
	OwnerFunds   bool        `json:"owner_funds"`
}

type SubmitRequest struct {
	Method string             `json:"method"`
	Params []SubmitParamEntry `json:"params"`
}

type SubmitParamEntry struct {
	TxBlob string `json:"tx_blob"`
}

type SubmitResponse struct {
}

type LedgerResponse struct {
	Result LedgerResult `json:"result"`
}

type LedgerResult struct {
	Ledger             LedgerInfo `json:"ledger"`
	LedgerCurrentIndex int        `json:"ledger_current_index"`
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
	Sequence           int             `json:"Sequence"`
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
	OwnerCount        int64  `json:"OwnerCount"`
	PreviousTxnID     string `json:"PreviousTxnID"`
	PreviousTxnLgrSeq int64  `json:"PreviousTxnLgrSeq"`
	Sequence          int    `json:"Sequence"`
	Index             string `json:"Index"`
}

func (client *Client) FetchBaseInput(ctx context.Context, args xcbuilder.TransferArgs) (xrptxinput.TxInput, error) {
	txInput := xrptxinput.NewTxInput()

	XRPTransaction := txInput.XRPTx

	XRPTransaction.Account = args.GetFrom()
	XRPTransaction.Destination = args.GetTo()
	XRPTransaction.Amount = args.GetAmount()
	XRPTransaction.TransactionType = xrptx.PAYMENT
	XRPTransaction.Fee = "12"
	//XRPTransaction.Flags = 0

	currentSequence, err := client.getNextValidSeqNumber(XRPTransaction.Account)
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	XRPTransaction.Sequence = *currentSequence

	ledgerSequence, err := client.getLatestValidatedLedgerSequence()
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	ledgerSequencePtr := *ledgerSequence
	ledgerOffset := 20
	lastLedgerSequence := ledgerSequencePtr + ledgerOffset
	XRPTransaction.LastLedgerSequence = lastLedgerSequence

	fmt.Println("SubmitTransaction2:", XRPTransaction)

	txInput.XRPTx = XRPTransaction

	return *txInput, nil
}

// FetchTransferInput returns tx input for a Template tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput, err := client.FetchBaseInput(ctx, args)
	if err != nil {
		return nil, err
	}

	return &txInput, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	txInput, err := client.FetchTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}

	return txInput, nil
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	serializedTxInput, err := txInput.Serialize()
	if err != nil {
		return err
	}

	submitRequest := &SubmitRequest{
		Method: Submit,
		Params: []SubmitParamEntry{
			{
				TxBlob: string(serializedTxInput),
			},
		},
	}

	var submitResponse SubmitResponse
	err = client.Send(MethodPost, submitRequest, &submitResponse)
	if err != nil {
		return err
	}

	return nil
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

	txRequest := &TransactionRequest{
		Method: Transaction,
		Params: []TransactionParamEntry{
			{
				Transaction: txHash,
				Binary:      false,
			},
		},
	}

	var txResponse TransactionResponse
	err := client.Send(MethodPost, txRequest, &txResponse)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	ledgerRequest := LedgerRequest{
		Method: Ledger,
		Params: []LedgerParamEntry{
			{
				LedgerIndex: "current",
			},
		},
	}

	var ledgerResponse LedgerResponse
	err = client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

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
		Confirmations: int64(confirmations),
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

	request := AccountInfoRequest{
		Method: AccountInfo,
		Params: []AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: Validated,
			},
		},
	}

	var accountInfoResponse AccountInfoResponse
	err := client.Send(MethodPost, request, &accountInfoResponse)
	if err != nil {
		return zero, err
	}

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

	request := AccountLinesRequest{
		Method: AccountLines,
		Params: []AccountLinesParamEntry{
			{
				Account: address,
			},
		},
	}

	var accountLinesResponse AccountLinesResponse
	err = client.Send(MethodPost, request, &accountLinesResponse)
	if err != nil {
		return zero, err
	}

	var balance string
	for _, line := range accountLinesResponse.Result.Lines {
		if line.Currency == asset && line.Account == contract {
			balance = line.Balance
		}
	}

	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	humanReadbleBalance, err := xc.NewAmountHumanReadableFromStr(balance)
	if err != nil {
		return zero, fmt.Errorf("failed to parse balance for account: %s", address)
	}
	return humanReadbleBalance.ToBlockchain(15), nil
}

func (client *Client) Send(method string, requestBody any, response any) error {

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	request, err := http.NewRequest(method, client.Url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch balance, HTTP status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
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

func (client *Client) getNextValidSeqNumber(address xc.Address) (*int, error) {
	request := AccountInfoRequest{
		Method: AccountInfo,
		Params: []AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: Validated,
			},
		},
	}

	var accountInfoResponse AccountInfoResponse
	err := client.Send(MethodPost, request, &accountInfoResponse)
	if err != nil {
		return nil, err
	}

	sequence := accountInfoResponse.Result.AccountData.Sequence
	return &sequence, nil
}

func (client *Client) getLatestValidatedLedgerSequence() (*int, error) {
	ledgerRequest := LedgerRequest{
		Method: Ledger,
		Params: []LedgerParamEntry{
			{
				LedgerIndex: Current,
			},
		},
	}

	var ledgerResponse LedgerResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	ledgerCurrentIndex := ledgerResponse.Result.LedgerCurrentIndex
	return &ledgerCurrentIndex, nil
}
