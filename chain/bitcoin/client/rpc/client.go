package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/types"
	"github.com/sirupsen/logrus"
)

type Client struct {
	httpClient *http.Client
	Url        string
	chain      *xc.ChainConfig
}

func NewClient(url string, chain *xc.ChainConfig) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		Url:        url,
		chain:      chain,
	}
}

func (client *Client) SetHttpClient(httpClient *http.Client) {
	client.httpClient = httpClient
}

// JSON RPC request structure
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSON RPC response structure
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	// decode ID as string or int
	ID     xc.AmountBlockchain `json:"id"`
	Result json.RawMessage     `json:"result,omitempty"`
	Error  *types.JsonRPCError `json:"error,omitempty"`
}

// call makes a JSON RPC call to the QuickNode endpoint
func (client *Client) call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON RPC request: %w", err)
	}

	log := logrus.WithFields(logrus.Fields{
		"url":    client.Url,
		"method": method,
		"params": params,
	})
	log.Trace("call json-rpc")

	req, err := http.NewRequestWithContext(ctx, "POST", client.Url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("JSON RPC call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.WithField("body", string(body)).WithField("status", resp.StatusCode).Trace("json-rpc response")
	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return fmt.Errorf("failed to unmarshal JSON RPC response (http status=%d): %w", resp.StatusCode, err)
	}

	if jsonResp.Error != nil {
		jsonResp.Error.HttpStatus = resp.StatusCode
		return jsonResp.Error
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.Unmarshal(jsonResp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}
