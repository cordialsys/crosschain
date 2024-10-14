package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xrp/address/contract"
	"github.com/cordialsys/crosschain/chain/xrp/client/events"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
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

func (client *Client) FetchBaseInput(ctx context.Context, args xcbuilder.TransferArgs) (xrptxinput.TxInput, error) {
	txInput := xrptxinput.NewTxInput()

	account := args.GetFrom()

	currentSequence, err := client.getNextValidSeqNumber(account)
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	currentSequencePtr := *currentSequence
	txInput.Sequence = currentSequencePtr

	ledgerSequence, err := client.getLatestValidatedLedgerSequence()
	if err != nil {
		return xrptxinput.TxInput{}, err
	}
	ledgerSequencePtr := *ledgerSequence
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

	ledgerRequest := types.LedgerRequest{
		Method: "ledger",
		Params: []types.LedgerParamEntry{
			{
				LedgerIndex: "current",
			},
		},
	}

	var ledgerResponse types.LedgerResponse
	err = client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	name := xclient.TransactionName(string(client.Asset.GetChain().Chain) + txResponse.Result.Hash)

	blockTime := time.Unix(types.XRP_EPOCH+txResponse.Result.Date, 0)

	block := xclient.NewBlock(uint64(txResponse.Result.LedgerIndex), txResponse.Result.Hash, blockTime)

	confirmations := ledgerResponse.Result.LedgerCurrentIndex - txResponse.Result.Sequence

	var errMsg *string
	if txResponse.Result.Status == "error" {
		msg := "transaction failed"
		errMsg = &msg
	}

	txInfo := xclient.TxInfo{
		Name:          name,
		Hash:          txResponse.Result.Hash,
		Chain:         client.Asset.GetChain().Chain,
		Block:         block,
		Transfers:     []*xclient.Transfer{},
		Fees:          []*xclient.Balance{},
		Confirmations: uint64(confirmations),
		Error:         errMsg,
	}

	affectedNodes := txResponse.Result.Meta.AffectedNodes
	tf := xclient.NewTransfer(client.Asset.GetChain().Chain)
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

		isSource, err := xrpNode.IsSource(&txResponse)
		if err != nil {
			return xclient.TxInfo{}, err
		}

		if isSource {
			tf.AddSource(
				address,
				contract,
				amount,
				nil,
			)
		} else {
			tf.AddDestination(
				address,
				contract,
				amount,
				nil,
			)
		}

	}

	txInfo.AddTransfer(tf)

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

func (client *Client) getLatestValidatedLedgerSequence() (*int64, error) {
	ledgerRequest := types.LedgerRequest{
		Method: "ledger",
		Params: []types.LedgerParamEntry{
			{
				LedgerIndex: types.Current,
			},
		},
	}

	var ledgerResponse types.LedgerResponse
	err := client.Send(MethodPost, ledgerRequest, &ledgerResponse)
	if err != nil {
		return nil, err
	}

	ledgerCurrentIndex := ledgerResponse.Result.LedgerCurrentIndex
	return &ledgerCurrentIndex, nil
}
