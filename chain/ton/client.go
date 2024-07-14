package ton

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/api"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
)

// Client for Template
type Client struct {
	Url   string
	Asset xc.ITask
}

var _ xclient.FullClient = &Client{}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	url := cfgI.GetChain().URL
	url = strings.TrimSuffix(url, "/")

	return &Client{url, cfgI}, nil
}

// Function to make HTTP GET request and handle response
func (cli *Client) get(path string, response any) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", cli.Url, path)
	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		if response != nil {
			if err := json.Unmarshal(body, response); err != nil {
				return fmt.Errorf("failed to unmarshal response: %v", err)
			}
		}
		return nil
	} else {
		// Deserialize to ErrorResponse struct for other status codes
		var errorResponse api.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		if errorResponse.Error != "" {
			return fmt.Errorf("%s", errorResponse.Error)
		}
		if len(errorResponse.Detail) > 0 {
			return fmt.Errorf("%s: %s", errorResponse.Detail[0].Type, errorResponse.Detail[0].Msg)
		}
		logrus.WithField("body", string(body)).WithField("chain", cli.Asset.GetChain().Chain).Warn("unknown ton error")
		return fmt.Errorf("unknown ton error (%d)", resp.StatusCode)
	}
}

// FetchTxInput returns tx input for a Template tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	return &TxInput{}, errors.New("not implemented")
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	return errors.New("not implemented")
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	return xc.LegacyTxInfo{}, errors.New("not implemented")
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	return xclient.TxInfo{}, errors.New("not implemented")
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	resp := &api.GetAccountResponse{}
	err := client.get(fmt.Sprintf("/api/v3/account?address=%s", address), resp)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	return xc.NewAmountBlockchainFromStr(resp.Balance), nil
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	if client.Asset.GetContract() == "" {
		return client.FetchNativeBalance(ctx, address)
	}
	resp := &api.JettonWalletsResponse{}
	err := client.get(fmt.Sprintf("/api/v3/jetton/wallets?owner_address=%s&jetton_address=%s", address, client.Asset.GetContract()), resp)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	sum := xc.NewAmountBlockchainFromUint64(0)
	for _, wallet := range resp.JettonWallets {
		bal := xc.NewAmountBlockchainFromStr(wallet.Balance)
		sum = sum.Add(&bal)
	}
	return sum, nil
}
