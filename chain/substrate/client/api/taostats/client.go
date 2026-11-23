package taostats

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ClientArgs struct {
	ApiKey string
}

type Client struct {
	baseUrl string
	apiKey  string
	limiter *rate.Limiter
}

func NewClient(baseUrl string, apiKey string, limiter *rate.Limiter) *Client {
	return &Client{baseUrl, apiKey, limiter}
}

func (client *Client) Get(ctx context.Context, url string, outputData any) error {
	err := client.limiter.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed wait on limiter: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	if client.apiKey != "" {
		req.Header.Add("Authorization", client.apiKey)
	}
	logrus.WithField("url", url).Debug("post request")

	explorerClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := explorerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logrus.WithField("body", string(body)).WithField("url", url).WithField("status", resp.StatusCode).Debug("post response")
	if resp.StatusCode != 200 {
		rpcErr := &Error{}
		err2 := json.Unmarshal(body, rpcErr)
		if err2 != nil || rpcErr.Message == "" {
			return fmt.Errorf("response failed (%d)", resp.StatusCode)
		}
		return fmt.Errorf("%s (%d)", rpcErr.Message, resp.StatusCode)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	err = decoder.Decode(outputData)
	if err != nil {
		return err
	}
	return err
}

func (client *Client) GetTransaction(ctx context.Context, txHash string) (*Extrinsic, error) {
	url := ""
	if _, _, err := api.BlockAndOffset(txHash).Parse(); err == nil {
		url = fmt.Sprintf("%s/v1/extrinsics?id=%s", client.baseUrl, txHash)
	} else {
		if !strings.HasPrefix(string(txHash), "0x") {
			txHash = "0x" + txHash
		}
		url = fmt.Sprintf("%s/v1/extrinsics?hash=%s", client.baseUrl, txHash)
	}
	var extResponse GetExtrinicResponse
	err := client.Get(ctx, url, &extResponse)
	if err != nil {
		return nil, fmt.Errorf("could not lookup extrinsic: %v", err)
	}
	if len(extResponse.Data) == 0 {
		return nil, fmt.Errorf("%s not found", txHash)
	}
	return &extResponse.Data[0], nil
}

func (client *Client) GetBlock(ctx context.Context, height int64) (*Block, error) {
	url := fmt.Sprintf("%s/v1/blocks?block_number=%d", client.baseUrl, height)
	var extResponse GetBlocksResponse
	err := client.Get(ctx, url, &extResponse)
	if err != nil {
		return nil, fmt.Errorf("could not lookup block %d: %v", height, err)
	}
	if len(extResponse.Data) == 0 {
		return nil, fmt.Errorf("block %d not found", height)
	}
	return &extResponse.Data[0], nil
}

func (client *Client) GetEvents(ctx context.Context, ext *Extrinsic) ([]*Event, error) {
	var events []*Event
	for page := 1; ; {
		endpoint := fmt.Sprintf("%s/v1/events?extrinsic_id=%s&limit=200&page=%d", client.baseUrl, url.QueryEscape(ext.ID), page)
		var response GetEventsResponse
		if err := client.Get(ctx, endpoint, &response); err != nil {
			return nil, fmt.Errorf("could not lookup events for %s: %w", ext.ID, err)
		}
		events = append(events, response.Data...)
		if response.Pagination.NextPage == nil {
			return events, nil
		}
		next := *response.Pagination.NextPage
		if next <= page {
			return nil, fmt.Errorf("invalid events pagination: next page %d after %d", next, page)
		}
		page = next
	}

}

func (client *Client) GetAccount(ctx context.Context, address string) (*AccountData, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s", client.baseUrl, url.PathEscape(address))
	var response AccountResponse
	if err := client.Get(ctx, endpoint, &response); err != nil {
		return nil, fmt.Errorf("could not lookup account %s: %w", address, err)
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("TAO address %s not found", address)
	}
	return &response.Data[0], nil
}
