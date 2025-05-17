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
	url   string
	chain xc.NativeAsset
}

func NewClient(url string, chain xc.NativeAsset) *Client {
	url = strings.TrimSuffix(url, "/")
	return &Client{url, chain}
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
	// if cli.ApiKey != "" {
	// 	request.Header.Add("authorization", "Bearer "+cli.ApiKey)
	// }
	log.Debug("sending request")
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
		// var errorResponse ErrorResponse
		logrus.WithField("body", string(body)).Debug("error")
		// if err := json.Unmarshal(body, &errorResponse); err != nil {
		// 	return fmt.Errorf("failed to unmarshal error response: %v", err)
		// }
		// if errorResponse.Message != "" {
		// 	return fmt.Errorf("%s", errorResponse.Message)
		// }
		logrus.WithField("body", string(body)).WithField("chain", cli.chain).Warn("unknown error")
		return fmt.Errorf("unknown error (%d)", resp.StatusCode)
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
