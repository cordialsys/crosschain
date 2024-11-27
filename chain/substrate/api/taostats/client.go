package taostats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cordialsys/crosschain/chain/substrate/api"
	"github.com/sirupsen/logrus"
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
}

func NewClient(baseUrl string, apiKey string) *Client {
	return &Client{baseUrl, apiKey}
}

func (client *Client) Get(ctx context.Context, url string, outputData any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	if client.apiKey != "" {
		req.Header.Add("Authorization", client.apiKey)
	}

	explorerClient := &http.Client{}
	resp, err := explorerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logrus.WithField("body", string(body)).WithField("url", url).WithField("status", resp.StatusCode).Debug("post")
	if resp.StatusCode != 200 {
		rpcErr := &Error{}
		err2 := json.Unmarshal(body, rpcErr)
		if err2 != nil || rpcErr.Message == "" {
			return fmt.Errorf("respones failed (%d)", resp.StatusCode)
		}
		return fmt.Errorf("%s (%d)", rpcErr.Message, resp.StatusCode)
	}
	err = json.Unmarshal(body, &outputData)
	if err != nil {
		return err
	}
	return err
}

func (client *Client) GetTransaction(ctx context.Context, txHash string) (*Extrinsic, error) {
	url := ""
	if _, _, err := api.BlockAndOffset(txHash).Parse(); err == nil {
		url = fmt.Sprintf("%s/api/extrinsic/v1?id=%s", client.baseUrl, txHash)
	} else {
		if !strings.HasPrefix(string(txHash), "0x") {
			txHash = "0x" + txHash
		}
		url = fmt.Sprintf("%s/api/extrinsic/v1?hash=%s", client.baseUrl, txHash)
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
	url := fmt.Sprintf("%s/api/block/v1?block_number=%d", client.baseUrl, height)
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
	// use the ID and not the hash
	url := fmt.Sprintf("%s/api/event/v1?extrinsic_id=%s", client.baseUrl, ext.ID)

	var extResponse GetEventsResponse
	err := client.Get(ctx, url, &extResponse)
	if err != nil {
		return nil, fmt.Errorf("could not lookup events for %s: %v", ext.ID, err)
	}
	return extResponse.Data, nil
}
