package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"
)

type Client struct {
	url        string
	chain      xc.NativeAsset
	httpClient *http.Client
}

func NewClient(url string, chain xc.NativeAsset, httpClient *http.Client) *Client {
	url = strings.TrimSuffix(url, "/")
	return &Client{url, chain, httpClient}
}

type ErrorResponse struct {
	Code     int    `json:"code"`
	Detail   string `json:"detail"`
	ErrorMsg string `json:"error"`
}

func (e *ErrorResponse) Error() string {
	if e.ErrorMsg != "" {
		return e.ErrorMsg
	}
	return fmt.Sprintf("%s (%d)", e.Detail, e.Code)
}

func (cli *Client) Do(method string, path string, requestBody any, response any) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", cli.url, path)
	var request *http.Request
	var err error
	if requestBody == nil {
		request, err = http.NewRequest(method, url, nil)
	} else {
		bz, _ := json.Marshal(requestBody)
		request, err = http.NewRequest(method, url, bytes.NewBuffer(bz))
		if err == nil {
			request.Header.Add("content-type", "application/json")
		}
	}
	if err != nil {
		return err
	}
	log := logrus.WithField("url", url).WithField("method", method)
	log.Debug("sending request")
	resp, err := cli.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"body":   string(body),
		"status": resp.StatusCode,
	}).Debug("response")

	if resp.StatusCode == http.StatusOK || resp.StatusCode == 201 {
		if response != nil {
			if err := json.Unmarshal(body, response); err != nil {
				return fmt.Errorf("failed to unmarshal response: %v", err)
			}
		}
		return nil
	} else {
		var errorResponse ErrorResponse
		errorResponse.Code = resp.StatusCode
		logrus.WithField("body", string(body)).Debug("error")
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return fmt.Errorf("unknown error (%d)", resp.StatusCode)
		}
		return &errorResponse
	}
}

// Get Utxos for addresses
func (cli *Client) GetUtxos(addresses []string) ([]UtxoResponse, error) {
	var requestBody = UtxoRequest{
		Addresses: &addresses,
	}
	var response []UtxoResponse

	if err := cli.Do("POST", "/addresses/utxos", &requestBody, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (cli *Client) SubmitTransaction(serializedSigned json.RawMessage) (SubmitTransactionResponse, error) {
	var response SubmitTransactionResponse

	if err := cli.Do("POST", "/transactions", serializedSigned, &response); err != nil {
		return response, err
	}
	return response, nil
}

func (cli *Client) GetTransaction(txId string) (*TxModel, error) {
	var response TxModel

	if err := cli.Do("GET", fmt.Sprintf("/transactions/%s?resolve_previous_outpoints=light", txId), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// The blue score is what we use as a proxy for the block height.
func (cli *Client) GetVirtualChainBlueScore() (*EndpointsGetVirtualChainBlueScoreBlockdagResponse, error) {
	var response EndpointsGetVirtualChainBlueScoreBlockdagResponse

	if err := cli.Do("GET", "/info/virtual-chain-blue-score", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (cli *Client) GetBlocksFromBlockScore(blockScore uint64) ([]*BlockModel, error) {
	var response []*BlockModel

	if err := cli.Do("GET", fmt.Sprintf("/blocks-from-bluescore?blueScore=%d&includeTransactions=true", blockScore), nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (cli *Client) GetFeeEstimate() (*FeeEstimateResponse, error) {
	var response FeeEstimateResponse
	if err := cli.Do("GET", "/info/fee-estimate", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
