package figment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/sirupsen/logrus"
)

type Client struct {
	Chain   string
	Url     string
	ApiKey  string
	Network string
}

type Error struct {
	Message string `json:"message"`
}

func NewClient(chain, network, url, apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api-key required")
	}

	return &Client{
		Chain:   chain,
		Url:     url,
		ApiKey:  apiKey,
		Network: network,
	}, nil
}

func (cli *Client) Post(path string, requestBody any, response any) error {
	return cli.Send("POST", path, requestBody, response)
}

func (cli *Client) Get(path string, response any) error {
	return cli.Send("GET", path, nil, response)
}

func (cli *Client) Send(method string, path string, requestBody any, response any) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", cli.Url, path)
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
	request.Header.Add("accept", "application/json")
	if cli.ApiKey != "" {
		request.Header.Add("x-api-key", cli.ApiKey)
	}
	logrus.WithField("url", url).Debug(method)
	resp, err := http.DefaultClient.Do(request)
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
		// Deserialize to ErrorResponse struct for other status codes
		var errorResponse Error
		logrus.WithField("body", string(body)).Debug("error")
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		if errorResponse.Message != "" {
			return fmt.Errorf("%s", errorResponse.Message)
		}
		logrus.WithField("body", string(body)).WithField("chain", cli.Chain).Warn("unknown kiln error")
		return fmt.Errorf("unknown kiln error (%d)", resp.StatusCode)
	}
}

func (cli *Client) CreateValidator(count int, withdrawalAddr string) (*CreateValidatorResponse, error) {
	var res CreateValidatorResponse
	err := cli.Post("ethereum/validators", &CreateValidatorRequest{
		Network:           cli.Network,
		ValidatorsCount:   count,
		WithdrawalAddress: withdrawalAddr,
	}, &res)
	return &res, err
}

func (cli *Client) GetValidator(validator string) (*GetValidatorResponse, error) {
	var res GetValidatorResponse
	err := cli.Get(fmt.Sprintf("ethereum/validators/%s?network=%s", address.Ensure0x(validator), cli.Network), &res)
	return &res, err
}
