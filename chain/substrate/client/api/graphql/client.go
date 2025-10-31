package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ClientArgs struct {
	ApiKey  string
	Limiter *rate.Limiter
}

func Post(ctx context.Context, url string, inputJson []byte, outputData any, args *ClientArgs) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(inputJson))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	if args.ApiKey != "" {
		req.Header.Add("X-API-Key", args.ApiKey)
	}
	if args.Limiter != nil {
		err := args.Limiter.Wait(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait on linter: %w", err)
		}
	}

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
