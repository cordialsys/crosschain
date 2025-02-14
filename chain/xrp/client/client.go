package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xrp/address/contract"
	"github.com/cordialsys/crosschain/chain/xrp/client/events"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/sirupsen/logrus"
)

// Client for XRP
type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
}

var _ xclient.FullClient = &Client{}
var _ xclient.ClientWithDecimals = &Client{}

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

func (client *Client) FetchBaseInput(ctx context.Context, args xcbuilder.TransferArgs) (xrptxinput.TxInput, error) {
	txInput := xrptxinput.NewTxInput()

	account := args.GetFrom()

	currentSequence, err := client.getNextValidSeqNumber(account)
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	currentSequencePtr := *currentSequence
	txInput.Sequence = currentSequencePtr

	ledger, err := client.getLatestLedger(false)
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	ledgerSequencePtr := ledger.Result.LedgerCurrentIndex
	ledgerOffset := int64(20) // Ledger offset
	lastLedgerSequence := ledgerSequencePtr + ledgerOffset
	txInput.LastLedgerSequence = lastLedgerSequence

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
	serializedTxInputBytes, err := txInput.Serialize()
	if err != nil {
		return err
	}

	serializedTxInputHex := hex.EncodeToString(serializedTxInputBytes)
	serializedTxInputHexBytes := []byte(serializedTxInputHex)

	submitRequest := &types.SubmitRequest{
		Method: "submit",
		Params: []types.SubmitParamEntry{
			{
				TxBlob: string(serializedTxInputHexBytes),
			},
		},
	}

	var submitResponse types.SubmitResponse
	err = client.Send(MethodPost, submitRequest, &submitResponse)
	if err != nil {
		return err
	}

	return nil
}

// FetchTxInfo Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	txInfo, err := client.GetTxInfo(ctx, txHash)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	return txInfo, nil
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	return xc.LegacyTxInfo{}, fmt.Errorf("unimplemented")
}

func (client *Client) GetTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	txRequest := &types.TransactionRequest{
		Method: "tx",
		Params: []types.TransactionParamEntry{
			{
				Transaction: txHash,
				Binary:      false,
			},
		},
	}

	var txResponse types.TransactionResponse
	err := client.Send(MethodPost, txRequest, &txResponse)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	if txResponse.Result.Hash == "" {
		return xclient.TxInfo{}, errors.TransactionNotFoundf("no transaction by hash '%s'", txHash)
	}

	ledger, err := client.getLatestLedger(false)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain().Chain
	name := xclient.NewTransactionName(chain, txResponse.Result.Hash)

	blockTime := time.Unix(types.XRP_EPOCH+txResponse.Result.Date, 0)

	block := xclient.NewBlock(chain, uint64(txResponse.Result.LedgerIndex), "", blockTime)

	confirmations := ledger.Result.LedgerCurrentIndex - txResponse.Result.Sequence

	var errMsg *string
	if txResponse.Result.Meta.TransactionResult != "tesSUCCESS" {
		msg := fmt.Sprintf("transaction failed: %s", txResponse.Result.Meta.TransactionResult)
		errMsg = &msg
	}

	txInfo := xclient.TxInfo{
		Name:          name,
		Hash:          txResponse.Result.Hash,
		XChain:        client.Asset.GetChain().Chain,
		Block:         block,
		Movements:     []*xclient.Movement{},
		Fees:          []*xclient.Balance{},
		Confirmations: uint64(confirmations),
		Error:         errMsg,
	}

	affectedNodes := txResponse.Result.Meta.AffectedNodes

	for _, node := range affectedNodes {
		xrpNode, ok, err := events.NewEvent(node)
		if !ok {
			// skip
			continue
		}
		if err != nil {
			return xclient.TxInfo{}, err
		}

		// Fetch address, contract and amount
		address, err := xrpNode.GetAddress(&txResponse)
		if err != nil {
			return xclient.TxInfo{}, err
		}

		contract, err := xrpNode.GetContract()
		if err != nil {
			return xclient.TxInfo{}, err
		}

		amount, err := xrpNode.GetAmount()
		if err != nil {
			return xclient.TxInfo{}, err
		}
		// XRP sometimes reports balances as negative
		amount = amount.Abs()

		movement := xclient.NewMovement(client.Asset.GetChain().Chain, contract)
		isSource, err := xrpNode.IsSource(&txResponse)
		if err != nil {
			return xclient.TxInfo{}, err
		}

		if isSource {
			movement.AddSource(
				address,
				amount,
				nil,
			)
		} else {
			movement.AddDestination(
				address,
				amount,
				nil,
			)
		}
		txInfo.AddMovement(movement)
	}
	// We coalesce since the 'events' from XRP do not include both sender and recipient.
	// So the raw transfers we added aren't very clear, and we can simplify by merging together
	// based on asset.
	txInfo.Coalesece()

	txInfo.Fees = txInfo.CalculateFees()

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

	request := types.AccountInfoRequest{
		Method: "account_info",
		Params: []types.AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: types.Validated,
			},
		},
	}

	var accountInfoResponse types.AccountInfoResponse
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

	asset, contract, err := contract.ExtractAssetAndContract(assetContract)
	if err != nil {
		return zero, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}

	request := types.AccountLinesRequest{
		Method: "account_lines",
		Params: []types.AccountLinesParamEntry{
			{
				Account: address,
			},
		},
	}

	var accountLinesResponse types.AccountLinesResponse
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
	return humanReadbleBalance.ToBlockchain(types.TRUSTLINE_DECIMALS), nil
}

type XrpError struct {
	Result struct {
		ErrorStatus  string `json:"error"`
		ErrorMessage string `json:"error_message"`
		ErrorCode    int    `json:"error_code"`
	} `json:"result"`
}

func (err *XrpError) Error() string {
	return fmt.Sprintf("%s: %s (code: %d)", err.Result.ErrorStatus, err.Result.ErrorMessage, err.Result.ErrorCode)
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
	logrus.WithField("method", method).WithField("params", string(jsonPayload)).Debug("request")

	resp, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch balance, HTTP status: %s", resp.Status)
	}
	bz, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var errMaybe XrpError
	_ = json.Unmarshal(bz, &errMaybe)
	if errMaybe.Result.ErrorStatus != "" || errMaybe.Result.ErrorMessage != "" {
		return &errMaybe
	}

	logrus.WithField("body", string(bz)).Debug("response")
	err = json.Unmarshal(bz, response)
	if err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

func (client *Client) getNextValidSeqNumber(address xc.Address) (*int64, error) {
	request := types.AccountInfoRequest{
		Method: "account_info",
		Params: []types.AccountInfoParamEntry{
			{
				Account:     address,
				LedgerIndex: types.Validated,
			},
		},
	}

	var accountInfoResponse types.AccountInfoResponse
	err := client.Send(MethodPost, request, &accountInfoResponse)
	if err != nil {
		return nil, err
	}

	sequence := accountInfoResponse.Result.AccountData.Sequence
	return &sequence, nil
}

func (client *Client) getLedger(index types.LedgerIndex, transactions bool) (*types.LedgerResponse, error) {
	ledgerRequest := types.LedgerRequest{
		Method: "ledger",
		Params: []types.LedgerParamEntry{
			{
				LedgerIndex:  index,
				Transactions: transactions,
			},
		},
	}

	var ledgerResponse types.LedgerResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	return &ledgerResponse, nil
}

func (client *Client) getLedgerData(index types.LedgerIndex) (*types.LedgerDataResponse, error) {
	ledgerRequest := types.LedgerDataRequest{
		Method: "ledger_data",
		Params: []types.LedgerDataParams{
			{
				LedgerIndex: index,
				Limit:       1,
			},
		},
	}

	var ledgerResponse types.LedgerDataResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	return &ledgerResponse, nil
}

func (client *Client) getLatestLedger(transactions bool) (*types.LedgerResponse, error) {
	return client.getLedger(types.Current, transactions)
}

// Pretty simple for XRP as it's always fixed.
func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return types.XRP_NATIVE_DECIMALS, nil
	}

	return types.TRUSTLINE_DECIMALS, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	var ledger *types.LedgerResponse
	var err error
	height, ok := args.Height()
	if !ok {
		ledger, err = client.getLatestLedger(true)
		if err != nil {
			return nil, err
		}

	} else {
		ledger, err = client.getLedger(types.LedgerIndex(fmt.Sprint(height)), true)
		if err != nil {
			return nil, err
		}
	}
	data, err := client.getLedgerData(types.LedgerIndex(ledger.Result.Ledger.LedgerIndex))
	if err != nil {
		return nil, err
	}

	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(
			client.Asset.GetChain().Chain,
			xc.NewAmountBlockchainFromStr(ledger.Result.Ledger.LedgerIndex).Uint64(),
			data.Result.LedgerHash,
			time.Unix(types.XRP_EPOCH+ledger.Result.Ledger.CloseTime, 0),
		),
		TransactionIds: ledger.Result.Ledger.Transactions,
	}

	return block, nil

}
