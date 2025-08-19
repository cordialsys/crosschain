package bbrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cordialsys/crosschain/chain/bitcoin/client/blockbook/types"
	"github.com/sirupsen/logrus"
)

type Client struct {
	httpClient *http.Client
	Url        string
}

var _ types.BlockBookClient = &Client{}

func NewClient(url string) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		Url:        url,
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
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSON RPC error structure
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON RPC error %d: %s", err.Code, err.Message)
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
	log.Trace("call blockbook json-rpc")

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

	log.WithField("body", string(body)).WithField("status", resp.StatusCode).Trace("blockbook json-rpc response")

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return fmt.Errorf("failed to unmarshal JSON RPC response: %w", err)
	}

	if jsonResp.Error != nil {
		return jsonResp.Error
	}

	if result != nil {
		if err := json.Unmarshal(jsonResp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}
