package client

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xlm/client/types"
	"github.com/cordialsys/crosschain/chain/xlm/common"
	xlminput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/stellar/go/xdr"
)

type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
	Passphrase string
}

var _ xclient.Client = &Client{}

func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	networkPassphrase := cfg.ChainID.AsString()
	if networkPassphrase == "" {
		return nil, fmt.Errorf("stellar configuration is missing chain-id")
	}

	if cfg.GasBudgetDefault.Decimal().InexactFloat64() <= 0 {
		return nil, fmt.Errorf("chain gas-budget-default should be set to value greater than 0.0")
	}
	budget := cfg.GasBudgetDefault.ToBlockchain(cfg.Decimals).Uint64()
	if budget >= math.MaxUint32 {
		return nil, fmt.Errorf("gas-budget-default exceeds uint32 type for XLM")
	}

	if cfg.TransactionActiveTime == 0 {
		return nil, fmt.Errorf("transaction-active-time should be greaterthan 0")
	}

	return &Client{
		Url:        cfg.URL,
		HttpClient: cfg.DefaultHttpClient(),
		Asset:      cfgI,
	}, nil
}

// FetchTransferInput returns tx input for a Stellar tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	config := client.Asset.GetChain()
	txInput := xlminput.NewTxInput(config.ChainID.AsString())
	account := args.GetFrom()
	accountDetails, err := client.FetchAccountDetails(account)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account details: %w", err)
	}

	currentSequence, err := strconv.ParseInt(accountDetails.Sequence, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sequence number: %w", err)
	}

	ledger, err := client.FetchLatestLedgerInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ledger info: %w", err)
	}

	txInput.Sequence = currentSequence + 1
	txInput.MinLedgerSequence = ledger.Sequence

	remainingBalance, err := accountDetails.GetNativeBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to read native balance: %w", err)
	}

	// Validate the amount and deduct it from the balance if the input
	// pertains to a native transaction
	if _, ok := args.GetContract(); !ok {
		amount := args.GetAmount()
		if remainingBalance.Cmp(&amount) == -1 {
			return nil, fmt.Errorf("failed to create tx input, tx amount(%s) greater than balance(%s)", amount.String(), remainingBalance.String())
		}
		remainingBalance = remainingBalance.Sub(&amount)
	}

	// Stellar requires the MaxFee specification, which defines the maximum amount
	// we are willing to spend on the transaction fee.
	maxFee := config.GasBudgetDefault.ToBlockchain(config.Decimals)

	// If balance is greater than blockchainFee, we can safely use specified MaxFee
	// Use remaining balance as a max fee otherwise
	if remainingBalance.Cmp(&maxFee) > 0 {
		txInput.MaxFee = uint32(maxFee.Uint64())
	} else {
		txInput.MaxFee = uint32(remainingBalance.Uint64())
	}

	txInput.TransactionActiveTime = config.TransactionActiveTime
	return txInput, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	txInput, err := client.FetchTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}
	return txInput, nil
}

// Broadcast a signed transaction to the chain
func (client *Client) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq) error {
	bytes, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(bytes)
	// Make sure that base64 string is properly escaped
	urlTx := url.QueryEscape(encoded)
	url := fmt.Sprintf("%s/transactions_async?tx=%s", client.Url, urlTx)

	var submitResult types.AsyncTxSubmissionResult
	if err := client.Post(url, nil, &submitResult); err != nil {
		return fmt.Errorf("failed to send post request: %w", err)
	}

	if submitResult.IsError() {
		if err := submitResult.DecodeErrorResultXdr(); err != nil {
			return fmt.Errorf("failed to decode error: %w", err)
		}
		return fmt.Errorf("failed to submit transaction: %w", &submitResult)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, fmt.Errorf("not implemented")
}

func (client *Client) FetchLatestLedgerInfo() (types.GetLedgerResult, error) {
	url := fmt.Sprintf("%s/ledgers?order=desc&limit=1", client.Url)
	var result types.GetLatestLedgerResult
	err := client.Get(url, &result)
	if err != nil {
		return types.GetLedgerResult{}, nil
	}

	if len(result.Embedded.Records) == 0 {
		return types.GetLedgerResult{}, fmt.Errorf("fetch latest ledger response empty")
	}

	return result.Embedded.Records[0], nil
}

func (client *Client) FetchLedgerInfo(sequence uint64) (types.GetLedgerResult, error) {
	url := fmt.Sprintf("%s/ledgers/%d", client.Url, sequence)
	var result types.GetLedgerResult
	err := client.Get(url, &result)
	return result, err
}

func (client *Client) FetchLedgerTransactions(sequence uint64, limit int, cursor string) (*types.GetLedgerTransactionResult, error) {
	url := fmt.Sprintf("%s/ledgers/%d/transactions?include_failed=true&limit=%d&cursor=%s", client.Url, sequence, limit, cursor)
	var result types.GetLedgerTransactionResult
	err := client.Get(url, &result)
	return &result, err
}

// Fetch ledger data and create xclient.TxInfo
func (client *Client) InitializeTxInfo(txHash xc.TxHash, transaction types.GetTransactionResult) (txinfo.TxInfo, error) {
	chainCfg := client.Asset.GetChain()
	chain := chainCfg.Chain
	sTxHash := string(txHash)
	// TODO: It works, but consider using proper ISO8601 parser
	time, err := time.Parse(time.RFC3339, transaction.CreatedAt)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	ledger, err := client.FetchLedgerInfo(transaction.Ledger)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get ledger (%v) data, error: %w", transaction.Ledger, err)
	}

	block := txinfo.NewBlock(chain, uint64(transaction.Ledger), ledger.Hash, time)
	var errMsg *string
	if !transaction.Successful {
		msg := "transaction failed"
		errMsg = &msg
	}

	latestLedger, err := client.FetchLatestLedgerInfo()
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get latest ledger data, error: %w", err)
	}

	confirmations := uint64(latestLedger.Sequence) - transaction.Ledger
	txInfo := txinfo.NewTxInfo(block, chainCfg, sTxHash, confirmations, errMsg)

	return *txInfo, nil
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHash := args.TxHash()
	url := fmt.Sprintf("%s/transactions/%s", client.Url, string(txHash))
	var response types.GetTransactionResult
	err := client.Get(url, &response)
	if err != nil {
		if queryErr, ok := err.(*types.QueryProblem); ok {
			if queryErr.Status == 404 {
				return txinfo.TxInfo{}, errors.TransactionNotFoundf("%v", err)
			}
		}
		return txinfo.TxInfo{}, fmt.Errorf("failed to send http request: %w", err)
	}

	decodedEnvelope, err := base64.StdEncoding.DecodeString(response.EnvelopeXdr)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to decode envelope: %w", err)
	}

	txInfo, err := client.InitializeTxInfo(txHash, response)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to create transaction info: %w", err)
	}

	var operations []xdr.Operation
	var envelope xdr.TransactionEnvelope
	if err := envelope.UnmarshalBinary([]byte(decodedEnvelope)); err != nil {
		if strings.Contains(err.Error(), "is not a valid OperationType") {
			// not an operation we recognize, so we'll omit it
		} else {
			return txinfo.TxInfo{}, fmt.Errorf("failed to unmarshal envelope XDR: %v", err)
		}
	} else {
		operations = envelope.Operations()
	}

	// Populate movements depending on operation type
	for _, operation := range operations {
		sourceAccountMaybe := operation.SourceAccount
		if sourceAccountMaybe == nil {
			// Use source account of the enveloping transaction if not present in the operation
			fromEnv := envelope.SourceAccount()
			sourceAccountMaybe = &fromEnv
		}
		sourceAccount := *sourceAccountMaybe

		// Regular transfer from SourceAccount to Destination
		payment, isPayment := operation.Body.GetPaymentOp()
		if isPayment {
			err := ProcessPayment(&txInfo, GetAssetCode(payment.Asset), sourceAccount, payment.Destination, payment.Amount)
			if err != nil {
				return txinfo.TxInfo{}, FailedToProceedPayment(err)
			}
		}

		// CreateAccount operation - this can be treated as a regular payment, because it involves the same movements
		createAccount, isCreateAccount := operation.Body.GetCreateAccountOp()
		if isCreateAccount {
			err := ProcessPayment(&txInfo, "XLM", sourceAccount, createAccount.Destination.ToMuxedAccount(), createAccount.StartingBalance)
			if err != nil {
				return txinfo.TxInfo{}, FailedToProceedPayment(err)
			}
		}

		// PathPayments involve different source and destination assets
		pathPaymentSend, isPathSend := operation.Body.GetPathPaymentStrictSendOp()
		if isPathSend {
			sendAsset := GetAssetCode(pathPaymentSend.SendAsset)
			destAsset := GetAssetCode(pathPaymentSend.DestAsset)
			err := ProcessPathPayment(
				&txInfo,
				sendAsset,
				destAsset,
				sourceAccount,
				pathPaymentSend.Destination,
				pathPaymentSend.SendAmount,
				pathPaymentSend.DestMin)
			if err != nil {
				return txinfo.TxInfo{}, FailedToProceedPathPayment(err)
			}
		}

		// PathPayments involve different source and destination assets
		pathPaymentReceive, isPathReceive := operation.Body.GetPathPaymentStrictReceiveOp()
		if isPathReceive {
			sendAsset := GetAssetCode(pathPaymentReceive.SendAsset)
			destAsset := GetAssetCode(pathPaymentReceive.DestAsset)
			err := ProcessPathPayment(
				&txInfo,
				sendAsset,
				destAsset,
				sourceAccount,
				pathPaymentReceive.Destination,
				pathPaymentReceive.SendMax,
				pathPaymentReceive.DestAmount)
			if err != nil {
				return txinfo.TxInfo{}, FailedToProceedPathPayment(err)
			}
		}
	}
	// cast the memo param to a simple string
	memo := ""
	if text, ok := envelope.Memo().GetText(); ok {
		memo = text
	} else if id, ok := envelope.Memo().GetId(); ok {
		memo = fmt.Sprintf("%d", id)
	} else if hash, ok := envelope.Memo().GetHash(); ok {
		memo = hex.EncodeToString(hash[:])
	} else if hash, ok := envelope.Memo().GetRetHash(); ok {
		memo = hex.EncodeToString(hash[:])
	}

	for _, movement := range txInfo.Movements {
		movement.Memo = memo
	}

	if txInfo.Error != nil && *txInfo.Error != "" {
		// clear the movements, as they are rolled back if transaction fails
		txInfo.Movements = []*txinfo.Movement{}
	}

	// Add Fee movement
	txAccount := envelope.SourceAccount()
	feeAccount, err := txAccount.GetAddress()
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get transaction account: %w", err)
	}

	feeAmount, err := xc.NewAmountHumanReadableFromStr(response.FeeCharged)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to parse fee charged: %w", err)
	}
	// FeeCharged is returned in "stroops", which is the smallest amount of lumen, so we can ignore the decimals
	xcFee := feeAmount.ToBlockchain(0)
	txInfo.AddFee(xc.Address(feeAccount), "", xcFee, nil)
	txInfo.Fees = txInfo.CalculateFees()

	return txInfo, nil
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalanceByAsset(address, "XLM")
}

// Fetch asset balance by asset code
func (client *Client) FetchBalanceByAsset(address xc.Address, assetID string) (xc.AmountBlockchain, error) {
	url := fmt.Sprintf("%s/accounts/%s", client.Url, string(address))
	var response types.GetAccountResult
	if err := client.Get(url, &response); err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch account balances: %w", err)
	}

	contractDetails, err := common.GetAssetAndIssuerFromContract(assetID)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to get asset details: %w", err)
	}

	for _, balance := range response.Balances {
		if balance.AssetType == types.AssetTypeLiquidityPoolShares {
			continue
		}

		if balance.AssetCode == contractDetails.AssetCode &&
			balance.AssetIssuer == string(contractDetails.Issuer) {

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

func (client *Client) FetchAccountDetails(address xc.Address) (types.GetAccountResult, error) {
	url := fmt.Sprintf("%s/accounts/%s", client.Url, string(address))
	var response types.GetAccountResult
	if err := client.Get(url, &response); err != nil {
		return response, fmt.Errorf("failed to fetch account data: %w", err)
	}

	return response, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	if contract, ok := args.Contract(); ok {
		return client.FetchBalanceByAsset(args.Address(), string(contract))
	} else {
		return client.FetchNativeBalance(ctx, args.Address())
	}
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return int(client.Asset.GetChain().GetDecimals()), nil
}

// Send a POST request
func (client *Client) Post(url string, requestBody any, response any) error {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

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
		var queryProblem types.QueryProblem
		if err := json.NewDecoder(resp.Body).Decode(&queryProblem); err != nil {
			return fmt.Errorf("failed to decode response body: %s", err)
		}

		return &queryProblem
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

// Conversion from xdr.Asset to xc.NativeAsset
func GetAssetCode(asset xdr.Asset) xc.NativeAsset {
	if asset.Type == xdr.AssetTypeAssetTypeNative {
		return xc.NativeAsset("XLM")
	}

	return xc.NativeAsset(fmt.Sprintf("%s-%s", asset.GetCode(), asset.GetIssuer()))
}

// Process payment like operation. This type of operations produce one movement containing source and destination.
func ProcessPayment(txInfo *txinfo.TxInfo, asset xc.NativeAsset, source xdr.MuxedAccount, destination xdr.MuxedAccount, amount xdr.Int64) error {
	if txInfo == nil {
		return fmt.Errorf("missing txInfo")
	}

	sourceAccount, err := source.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get source account: %w", err)
	}

	destinationAccount, err := destination.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get destination account: %w", err)
	}

	movement := txinfo.NewMovement(xc.NativeAsset(asset), "")
	xcAmount, ok := xc.NewAmountBlockchainFromInt64(int64(amount))
	if !ok {
		return fmt.Errorf("failed to construct new blockchain amount from: %v", int64(amount))
	}
	movement.AddSource(xc.Address(sourceAccount), xcAmount, nil)
	movement.AddDestination(xc.Address(destinationAccount), xcAmount, nil)

	txInfo.AddMovement(movement)
	return nil
}

// Process cross asset payments. This type of operation produce two movements: one for source account and one for destination account
func ProcessPathPayment(txInfo *txinfo.TxInfo, sourceAsset xc.NativeAsset, destinationAsset xc.NativeAsset, source xdr.MuxedAccount, destination xdr.MuxedAccount, sourceAmount xdr.Int64, destinationAmount xdr.Int64) error {
	if txInfo == nil {
		return fmt.Errorf("missing txInfo")
	}

	sourceAccount, err := source.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get source account: %w", err)
	}

	destinationAccount, err := destination.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get destination account: %w", err)
	}

	xcSourceAmount, ok := xc.NewAmountBlockchainFromInt64(int64(sourceAmount))
	if !ok {
		return fmt.Errorf("failed to construct new blockchain amount from: %v", int64(sourceAmount))
	}
	sourceMovement := txinfo.NewMovement(sourceAsset, "")
	sourceMovement.AddSource(xc.Address(sourceAccount), xcSourceAmount, nil)
	txInfo.AddMovement(sourceMovement)

	xcDestinationAmount, ok := xc.NewAmountBlockchainFromInt64(int64(destinationAmount))
	if !ok {
		return fmt.Errorf("failed to construct new blockchain amount from: %v", int64(destinationAmount))
	}
	destinationMovement := txinfo.NewMovement(destinationAsset, "")
	destinationMovement.AddDestination(xc.Address(destinationAccount), xcDestinationAmount, nil)
	txInfo.AddMovement(destinationMovement)

	return nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	var ledger types.GetLedgerResult
	var err error
	height, ok := args.Height()
	if !ok {
		ledger, err = client.FetchLatestLedgerInfo()
	} else {
		ledger, err = client.FetchLedgerInfo(height)
	}
	if err != nil {
		return nil, err
	}
	time, err := time.Parse(time.RFC3339, ledger.ClosedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block timestamp: %w", err)
	}
	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			uint64(ledger.Sequence),
			ledger.Hash,
			time,
		),
	}

	const maxPageSize = 200
	cursor := ""
	// page through max 25 pages
	for range 25 {
		ledgerTxs, err := client.FetchLedgerTransactions(uint64(ledger.Sequence), maxPageSize, cursor)
		if err != nil {
			return nil, err
		}
		for _, tx := range ledgerTxs.Embedded.Records {
			block.TransactionIds = append(block.TransactionIds, tx.Hash)
			cursor = tx.PagingToken
		}
		if len(ledgerTxs.Embedded.Records) < maxPageSize {
			break
		}
	}

	return block, nil
}

func FailedToProceedPayment(err error) error {
	return fmt.Errorf("failed to process payment: %w", err)
}

func FailedToProceedPathPayment(err error) error {
	return fmt.Errorf("failed to process path payment: %w", err)
}
