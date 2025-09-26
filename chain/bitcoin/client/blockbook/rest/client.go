package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

func (client *Client) get(ctx context.Context, path string, resp interface{}) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", client.Url, path)
	log := logrus.WithFields(logrus.Fields{
		"url": url,
	})
	log.Trace("blockbook get")
	res, err := client.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("blockbook get failed: %w", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		var errResponse types.ErrorResponse
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			return &types.ErrorResponse{
				ErrorMessage: fmt.Sprintf("failed to get %s: code=%d", path, res.StatusCode),
				HttpStatus:   res.StatusCode,
			}
		}
		errResponse.HttpStatus = res.StatusCode
		return &errResponse
	}

	if resp != nil {
		err = json.Unmarshal(body, &resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *Client) post(ctx context.Context, path string, contentType string, input []byte, resp interface{}) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", client.Url, path)
	log := logrus.WithFields(logrus.Fields{
		"url":  url,
		"body": string(input),
	})
	log.Trace("blockbook post")
	res, err := client.httpClient.Post(url, contentType, bytes.NewReader(input))
	if err != nil {
		return fmt.Errorf("blockbook post failed: %w", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		var errResponse types.ErrorResponse
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			return &types.ErrorResponse{
				ErrorMessage: fmt.Sprintf("failed to get %s: code=%d", path, res.StatusCode),
				HttpStatus:   res.StatusCode,
			}
		}
		errResponse.HttpStatus = res.StatusCode
		return &errResponse
	}

	if resp != nil {
		err = json.Unmarshal(body, &resp)
		if err != nil {
			return err
		}
	}
	return nil
}
