package client

import (
	"bytes"
	"context"
	"strconv"

	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"

	// "github.com/cordialsys/crosschain/chain/xrp/address/contract"
	// "github.com/cordialsys/crosschain/chain/xrp/client/events"
	"github.com/cordialsys/crosschain/chain/xlm/client/types"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/stellar/go/xdr"
	//"github.com/stellar/go/gxdr"
)

const (
	jsonrpcVersion = "2.0"
	requestId      = 0
)

type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
}

var _ xclient.FullClient = &Client{}

// var _ xclient.ClientWithDecimals = &Client{}

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

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	params := types.GetTransactionParams{Hash: txHash}
	txRequest := types.NewTransactionRequest(params)
	var response types.GetTransactionResult
	err := client.Send(txRequest, &response)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to send http request: %w", err)
	}

	decodedEnvelope, err := base64.StdEncoding.DecodeString(response.EnvelopeXdr)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to decode envelope: %w", err)
	}

	var envelope xdr.TransactionEnvelope
	if err := envelope.UnmarshalBinary([]byte(decodedEnvelope)); err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to unmarshal envelope XDR: %e", err)
	}

	// decodedTransactionResult, err := base64.StdEncoding.DecodeString(response.ResultXdr)
	// if err != nil {
	// 	return xclient.TxInfo{}, fmt.Errorf("failed to decode result: %w", err)
	// }
	// var txResult xdr.TransactionResult
	// if err := txResult.UnmarshalBinary([]byte(decodedEnvelope)); err != nil {
	// 	return xclient.TxInfo{}, fmt.Errorf("failed to unmarshal transaction result XDR", err)
	// }

	chain := client.Asset.GetChain().Chain
	sTxHash := string(txHash)
	name := xclient.NewTransactionName(chain, sTxHash)
	timestamp, err := strconv.ParseInt(response.CreatedAt, 10, 64)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	blockTime := time.Unix(timestamp, 0)
	// TODO: Replace sTxHash with LedgerHash
	block := xclient.NewBlock(chain, uint64(response.Ledger), sTxHash, blockTime)
	transaction := types.Transaction {
		SourceAccount: envelope.SourceAccount().ToAccountId().GoString(),
		Fee: envelope.Fee(),
		SeqNum: envelope.SeqNum(),
		Operations: envelope.Operations(),
		Signatures: envelope.Signatures(),
	}
	fmt.Printf("\nResponse: %v\n\n", transaction)

	var errMsg *string
	if response.Status == "FAILED" {
		msg := "transaction failed"
		errMsg = &msg
	}

	txInfo := xclient.TxInfo{
		Name: name,
		Hash: sTxHash,
		XChain: chain,
		Block: block,
		Error: errMsg,
	}

	return txInfo, nil
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return xc.AmountBlockchain{}, errors.New("not implemented")
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return xc.AmountBlockchain{}, errors.New("not implemented")
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 0, errors.New("not implemented")
}

func (client *Client) Send(requestBody types.RPCRequest, response any) error {
	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	request, err := http.NewRequest(MethodPost, client.Url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	request.Header.Add("Content-Type", "application/json")

	resp, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch balance, HTTP status: %s", resp.Status)
	}

	wrappedResponse := types.RPCResponse{}
	wrappedResponse.Result = response

	if err := json.NewDecoder(resp.Body).Decode(&wrappedResponse); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

const MethodPost string = "POST"
