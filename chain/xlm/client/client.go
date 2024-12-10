package client

import (
	"bytes"
	"context"
	"encoding/json"
	// "encoding/hex"
	// "encoding/json"
	"errors"
	"fmt"
	"net/http"
	// "time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"

	// "github.com/cordialsys/crosschain/chain/xrp/address/contract"
	// "github.com/cordialsys/crosschain/chain/xrp/client/events"
	"github.com/cordialsys/crosschain/chain/xlm/client/types"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	// "github.com/sirupsen/logrus"
)

const (
	jsonrpcVersion = "2.0"
	requestId = 0
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
        Url: cfg.URL,
        HttpClient: http.DefaultClient,
        Asset: cfgI,
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
	params := types.GetTransactionParams{ Hash: txHash }
	txRequest := types.NewTransactionRequest(params)
	return xclient.TxInfo{}, errors.New("not implemented")
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

func (client *Client) Send(method string, requestBody types.RPCRequest, response any) error {
	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	request, err := http.NewRequest(MethodPost, client.Url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	return errors.New("not implemented")
}

const MethodPost string = "POST"
