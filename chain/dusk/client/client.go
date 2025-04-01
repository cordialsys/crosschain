package client

import (
	"bytes"
	"context"
	"strings"

	"encoding/json"
	"fmt"
	"io"
	"net/http"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	types "github.com/cordialsys/crosschain/chain/dusk/client/types"
	"github.com/cordialsys/crosschain/chain/dusk/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	log "github.com/sirupsen/logrus"
	"github.com/stellar/go/support/time"
)

// Client for Dusk
type Client struct {
	Asset      xc.ITask
	Url        string
	RuesUrl    string
	HttpClient *http.Client
	Logger     *log.Entry
}

var _ xclient.Client = &Client{}

// NewClient returns a new Dusk Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	logger := log.WithFields(log.Fields{
		"chain":   cfg.Chain,
		"rpc":     cfg.URL,
		"network": cfg.Network,
	})

	return &Client{
		Asset:      cfgI,
		Url:        cfg.GetChain().URL,
		RuesUrl:    fmt.Sprintf("%s/on", cfg.GetChain().URL),
		HttpClient: http.DefaultClient,
		Logger:     logger,
	}, nil
}

// FetchTransferInput returns tx input for a Dusk tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	accountStatus, err := client.FetchAccountStatus(args.GetFrom())
	if err != nil {
		return nil, err
	}

	// Calculate default fee_limit
	maxFee := client.Asset.GetChain().FeeLimit.ToBlockchain(client.Asset.GetDecimals())
	gasPrice := xc.NewAmountBlockchainFromUint64(tx_input.DEFAULT_GAS_PRICE)
	gasLimit := tx_input.EstimateFeeLimit(maxFee, gasPrice)

	return &tx_input.TxInput{
		Nonce:         accountStatus.Nonce + 1,
		GasLimit:      gasLimit.Uint64(),
		GasPrice:      gasPrice.Uint64(),
		RefundAccount: args.GetFrom(),
	}, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx verifies tx with `on/transactions/preverify` endpoint and then propagates it to the network
// using `on/transactions/propagate` endpoint.
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	bytes, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}

	verifyRequest := types.RuesRequest{
		Method: types.POST,
		Target: types.TARGET_TRANSACTIONS,
		Topic:  types.TOPIC_VERIFY,
		Params: bytes,
	}

	var verifyResponse string
	err = Request(client, verifyRequest, &verifyResponse)
	if err != nil {
		return fmt.Errorf("failed to submit tx: %w", err)
	}
	if verifyResponse != "" {
		return fmt.Errorf("failed to verify tx: %s", verifyResponse)
	}

	propagateRequest := types.RuesRequest{
		Method: types.POST,
		Target: types.TARGET_TRANSACTIONS,
		Topic:  types.TOPIC_PROPAGATE,
		Params: bytes,
	}

	var propagateResponse string
	err = Request(client, propagateRequest, &propagateResponse)
	if err != nil {
		if strings.Contains(err.Error(), "this transaction exists in the mempool") {
			return errors.TransactionExistsf("%v", err)
		}
		return fmt.Errorf("failed to submit tx: %w", err)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	return xc.LegacyTxInfo{}, fmt.Errorf("not implemented")
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	latestHeight, err := client.FetchLatestBlockHeight()
	if err != nil {
		return xclient.TxInfo{}, err
	}

	params := types.GetTransactionParams{
		Id: string(txHash),
	}
	request := types.NewGraphQlRequest(params.ToBytesParams(), "5d20d802b7a574c3316a103c02bb58b7")
	var response types.GetTransactionResult
	err = Request(client, request, &response)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	if response.SpentTransaction == nil {
		return xclient.TxInfo{}, fmt.Errorf("transaction not found: %s", txHash)
	}

	chain := client.Asset.GetChain().Chain
	blockTime := time.MillisFromInt64(response.SpentTransaction.BlockTimestamp)
	block := xclient.NewBlock(chain, response.SpentTransaction.BlockHeight, response.SpentTransaction.BlockHash, blockTime.ToTime())
	txInfo := xclient.NewTxInfo(block, client.Asset.GetChain(), response.SpentTransaction.ID, latestHeight-block.Height.Uint64(), &response.SpentTransaction.Err)

	transaction, err := response.SpentTransaction.Tx.GetTransaction()
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get transaction: %w", err)
	}
	sourceAddress := xc.Address(transaction.Sender)
	destinationAddress := xc.Address(transaction.Receiver)
	amount := xc.NewAmountBlockchainFromUint64(transaction.Value)
	movement := xclient.NewMovement(chain, "")
	movement.AddSource(sourceAddress, amount, nil)
	movement.AddDestination(destinationAddress, amount, nil)
	txInfo.AddMovement(movement)

	gasPrice := xc.NewAmountBlockchainFromStr(transaction.Fee.GasPrice)
	gasUsed := xc.NewAmountBlockchainFromUint64(response.SpentTransaction.GasSpent)
	feePrice := gasPrice.Mul(&gasUsed)
	txInfo.AddFee(sourceAddress, "", feePrice, nil)
	txInfo.Fees = txInfo.CalculateFees()
	txInfo.Final = int(txInfo.Confirmations) > client.Asset.GetChain().ConfirmationsFinal

	return *txInfo, nil
}

func (client *Client) FetchAccountStatus(address xc.Address) (types.GetAccountStatusResult, error) {
	request := types.RuesRequest{
		Method: types.POST,
		Target: types.TARGET_ACCOUNT,
		Entity: string(address),
		Topic:  types.TOPIC_STATUS,
		Params: nil,
	}

	var response types.GetAccountStatusResult
	err := Request(client, request, &response)
	if err != nil {
		return types.GetAccountStatusResult{}, fmt.Errorf("failed to get account status: %w", err)
	}

	return response, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	accountStatus, err := client.FetchAccountStatus(args.Address())
	if err != nil {
		return xc.AmountBlockchain{}, err
	}

	amount := xc.NewAmountBlockchainFromUint64(accountStatus.Balance)
	return amount, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return int(client.Asset.GetChain().GetDecimals()), nil
}

func (client *Client) FetchLatestBlockHeight() (uint64, error) {
	params := &types.GetLastBlockParams{}
	request := types.NewGraphQlRequest(params.ToBytesParams(), "5d20d802b7a574c3316a103c02bb58b7")
	var response types.LastBlockPairResult
	err := Request(client, request, &response)
	if err != nil {
		return 0, err
	}

	fh, ok := response.LastBlockPair.Json.LastFinalizedBlock[0].(float64)
	if !ok {
		return 0, fmt.Errorf("failed to get last finalized block height: %w", err)
	}

	return uint64(fh), nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, ok := args.Height()
	// Fetch latest finalized block height if `height` arg was not specified
	if !ok {
		h, err := client.FetchLatestBlockHeight()
		if err != nil {
			return nil, err
		}

		height = h
	}

	params := &types.GetBlockParams{
		Height: height,
	}
	request := types.NewGraphQlRequest(params.ToBytesParams(), "5d20d802b7a574c3316a103c02bb58b7")
	var response types.GetBlockResult
	err := Request(client, request, &response)
	if err != nil {
		return nil, err
	}

	block := response.Block
	blockTimestamp := time.MillisFromInt64(block.Header.Timestamp)
	xBlock := xclient.NewBlock(client.Asset.GetChain().Chain, block.Header.Height, block.Header.Hash, blockTimestamp.ToTime())
	transactions := make([]string, 0, len(block.Transactions))
	for _, tx := range block.Transactions {
		transactions = append(transactions, tx.Id)
	}

	return &xclient.BlockWithTransactions{
		Block:          *xBlock,
		TransactionIds: transactions,
	}, nil
}

// Send Rues request to the Dusk network
func Request(client *Client, rr types.RuesRequest, resp interface{}) error {
	logger := log.WithFields(log.Fields{
		"target": rr.Target,
		"entity": rr.Entity,
		"topic":  rr.Topic,
	})
	url := rr.GetUrl(client.RuesUrl)

	params := rr.GetParams()
	request, err := http.NewRequest(rr.Method, url, bytes.NewBuffer(params))
	logger.WithField("params", string(params)).Debug("sending request")
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if rr.IsGraphQL() {
		request.Header.Set("Content-Type", "application/json")
	} else {
		request.Header.Set("Content-Type", "application/octet-stream")
	}

	response, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()

	buff, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Trim `Rusk` node backtrace from the response body - it is noisy and not useful
	stackBacktraceIdx := strings.Index(string(buff), "Stack backtrace")
	if stackBacktraceIdx != -1 {
		buff = buff[:stackBacktraceIdx]
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send request, status: %s, error: %s", response.Status, buff)
	}

	logger.WithFields(log.Fields{
		"responseBody": string(buff),
		"status":       response.Status,
		"code":         response.StatusCode,
	}).Debug("got response")

	if len(buff) == 0 {
		return nil
	}

	switch resp.(type) {
	case string:
		resp = string(buff)
	default:
		err = json.Unmarshal(buff, resp)
		if err != nil {
			return fmt.Errorf("failed to decode response body: %w, buff: %s", err, string(buff))
		}
	}

	return nil
}
