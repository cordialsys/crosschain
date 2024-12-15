package client

import (
	"bytes"
	"context"

	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"

	"github.com/cordialsys/crosschain/chain/xlm/client/types"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/stellar/go/xdr"
)

type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
}

var _ xclient.FullClient = &Client{}
var _ xclient.ClientWithDecimals = &Client{}

func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()

	return &Client{
		Url:        cfg.URL,
		HttpClient: http.DefaultClient,
		Asset:      cfgI,
	}, nil
}

// FetchTransferInput returns tx input for a Template tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	return &xrptxinput.TxInput{}, errors.New("not implemented")
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	return nil, errors.New("not implemented")
}

// Broadcast a signed transaction to the chain
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	return errors.New("not implemented")
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	return xc.LegacyTxInfo{}, errors.New("not implemented")
}

func (client *Client) FetchLedgerInfo(sequence uint64) (types.GetLedgerResult, error) {
	url := client.GetLedger(sequence)
	var result types.GetLedgerResult
	err := client.Get(url, &result)
	return result, err
}

// Fetch ledger data and create xclient.TxInfo
func (client *Client) InitializeTxInfo(txHash xc.TxHash, transaction types.GetTransactionResult) (xclient.TxInfo, error) {
	chain := client.Asset.GetChain().Chain
	sTxHash := string(txHash)
	name := xclient.NewTransactionName(chain, sTxHash)
	// TODO: It works, but consider using proper ISO8601 parser
	time, err := time.Parse(time.RFC3339, transaction.CreatedAt)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	ledger, err := client.FetchLedgerInfo(transaction.Ledger)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get ledger data: %w", err)
	}

	block := xclient.NewBlock(chain, uint64(transaction.Ledger), ledger.Hash, time)
	var errMsg *string
	if transaction.Successful != true {
		msg := "transaction failed"
		errMsg = &msg
	}

	confirmations := ledger.Sequence - transaction.Ledger
	txInfo := xclient.TxInfo{
		Name:          name,
		Hash:          sTxHash,
		XChain:        chain,
		Block:         block,
		Error:         errMsg,
		Confirmations: confirmations,
	}
	return txInfo, nil
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	url := client.GetTransactionUrl(string(txHash))
	var response types.GetTransactionResult
	err := client.Get(url, &response)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to send http request: %w", err)
	}

	decodedEnvelope, err := base64.StdEncoding.DecodeString(response.EnvelopeXdr)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to decode envelope: %w", err)
	}

	txInfo, err := client.InitializeTxInfo(txHash, response)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to create transaction ifno: %w", err)
	}

	var envelope xdr.TransactionEnvelope
	if err := envelope.UnmarshalBinary([]byte(decodedEnvelope)); err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to unmarshal envelope XDR: %e", err)
	}

	// Populate movements depending on operation type
	for _, operation := range envelope.Operations() {

		// Regular transfer from SourceAccount to Destination
		payment, isPayment := operation.Body.GetPaymentOp()
		if isPayment {
			ProcessPayment(&txInfo, GetAssetCode(payment.Asset), *operation.SourceAccount, payment.Destination, payment.Amount)
		}

		// CreateAccount operation - this can be treated as a regular payment, because it involves the same movements
		createAccount, isCreateAccount := operation.Body.GetCreateAccountOp()
		if isCreateAccount {
			ProcessPayment(&txInfo, "XLM", *operation.SourceAccount, createAccount.Destination.ToMuxedAccount(), createAccount.StartingBalance)
		}

		// PathPayments involve differenc source and destination assets
		pathPaymentSend, isPathSend := operation.Body.GetPathPaymentStrictSendOp()
		if isPathSend {
			sendAsset := GetAssetCode(pathPaymentSend.SendAsset)
			destAsset := GetAssetCode(pathPaymentSend.DestAsset)
			ProcessPathPayment(
				&txInfo,
				sendAsset,
				destAsset,
				*operation.SourceAccount,
				pathPaymentSend.Destination,
				pathPaymentSend.SendAmount,
				pathPaymentSend.DestMin)
		}

		// PathPayments involve differenc source and destination assets
		pathPaymentReceive, isPathReceive := operation.Body.GetPathPaymentStrictReceiveOp()
		if isPathReceive {
			sendAsset := GetAssetCode(pathPaymentReceive.SendAsset)
			destAsset := GetAssetCode(pathPaymentReceive.DestAsset)
			ProcessPathPayment(
				&txInfo,
				sendAsset,
				destAsset,
				*operation.SourceAccount,
				pathPaymentReceive.Destination,
				pathPaymentReceive.SendMax,
				pathPaymentReceive.DestAmount)
		}
	}

	// Add Fee movement
	txAccount := envelope.SourceAccount()
	feeAccount, err := txAccount.GetAddress()
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get transaction account: %w", err)
	}

	feeAmount, err := xc.NewAmountHumanReadableFromStr(response.FeeCharged)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to parse fee charged: %w", err)
	}
	// FeeCharged is returned in "stroops", which is the smallest amount of lumen, so we can ignore the decimals
	xcFee := feeAmount.ToBlockchain(0)
	txInfo.AddFee(xc.Address(feeAccount), "", xcFee, nil)
	txInfo.Fees = txInfo.CalculateFees()

	return txInfo, nil
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalanceByAsset(address, true, xc.NativeAsset("XLM"))
}

// Fetch asset balance by asset code
func (client *Client) FetchBalanceByAsset(address xc.Address, fetchNative bool, asset xc.NativeAsset) (xc.AmountBlockchain, error) {
	url := client.GetAccountUrl(string(address))
	var response types.GetAccountResult
	if err := client.Get(url, &response); err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch account balances: %w", err)
	}

	// Asset code is omited for native currency
	if asset == "XLM" {
		asset = ""
	}
	for _, balance := range response.Balances {
		if balance.AssetType == types.AssetTypeLiquidityPoolShares {
			continue
		}

		if balance.AssetCode == string(asset) {
			readableAmount, err := xc.NewAmountHumanReadableFromStr(balance.Balance)
			if err != nil {
				return xc.AmountBlockchain{}, fmt.Errorf("failed to read balance decimal: %w", err)
			}
			blockchainAmount := readableAmount.ToBlockchain(client.Asset.GetChain().GetDecimals())
			return blockchainAmount, nil
		}
	}
	return xc.AmountBlockchain{}, nil
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalanceByAsset(address, true, client.Asset.GetChain().Chain)
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 0, errors.New("not implemented")
}

func (client *Client) GetTransactionUrl(txHash string) string {
	return fmt.Sprintf("%s/transactions/%s", client.Url, txHash)
}

func (client *Client) GetAccountUrl(address string) string {
	return fmt.Sprintf("%s/accounts/%s", client.Url, address)
}

func (client *Client) GetLedger(sequence uint64) string {
	return fmt.Sprintf("%s/ledgers/%d", client.Url, sequence)
}

// Send a POST request
func (client *Client) Post(url string, requestBody any, response any) error {
	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	resp, err := client.HttpClient.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to post, HTTP status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

// Send a GET request
func (client *Client) Get(url string, response any) error {
	resp, err := client.HttpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get, HTTP status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

// Conversion from xdr.Asset to xc.NativeAsset
func GetAssetCode(asset xdr.Asset) xc.NativeAsset {
	code := asset.GetCode()
	// Native ("XLM") is used if xdr.Asset is ""
	if code == "" {
		code = "XLM"
	}
	return xc.NativeAsset(code)
}

// Process payment like operation. This type of operations produce one movement containing source and destination.
func ProcessPayment(txInfo *xclient.TxInfo, asset xc.NativeAsset, source xdr.MuxedAccount, destination xdr.MuxedAccount, amount xdr.Int64) error {
	if txInfo == nil {
		return errors.New("missing txInfo")
	}

	sourceAccount, err := source.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get source account: %w", err)
	}

	destinationAccount, err := destination.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get destination account: %w", err)
	}

	movement := xclient.NewMovement(xc.NativeAsset(asset), "")
	xcAmount := xc.NewAmountBlockchainFromInt64(int64(amount))
	movement.AddSource(xc.Address(sourceAccount), xcAmount, nil)
	movement.AddDestination(xc.Address(destinationAccount), xcAmount, nil)

	txInfo.AddMovement(movement)
	return nil
}

// Process cross asset payments. This type of operation produce two movements: one for source account and one for destination account
func ProcessPathPayment(txInfo *xclient.TxInfo, sourceAsset xc.NativeAsset, destinationAsset xc.NativeAsset, source xdr.MuxedAccount, destination xdr.MuxedAccount, sourceAmount xdr.Int64, destinationAmount xdr.Int64) error {
	if txInfo == nil {
		return errors.New("missing txInfo")
	}

	sourceAccount, err := source.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get source account: %w", err)
	}

	destinationAccount, err := destination.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get destination account: %w", err)
	}

	xcSourceAmount := xc.NewAmountBlockchainFromInt64(int64(sourceAmount))
	sourceMovement := xclient.NewMovement(sourceAsset, "")
	sourceMovement.AddSource(xc.Address(sourceAccount), xcSourceAmount, nil)
	txInfo.AddMovement(sourceMovement)

	xcDestinationAmount := xc.NewAmountBlockchainFromInt64(int64(destinationAmount))
	destinationMovement := xclient.NewMovement(destinationAsset, "")
	destinationMovement.AddDestination(xc.Address(destinationAccount), xcDestinationAmount, nil)
	txInfo.AddMovement(destinationMovement)

	return nil
}
