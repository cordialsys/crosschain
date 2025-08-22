package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/client/types"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/sirupsen/logrus"
)

const (
	Hype         = "HYPE"
	HypeContract = xc.ContractAddress("0x0d01dc56dcaaca66ad901c959b4011ec")
	HypeDecimals = 8
)

// Client for hyperliquid
type Client struct {
	Asset      xc.ITask
	Url        *url.URL
	HttpClient *http.Client
}

var _ xclient.Client = &Client{}

// NewClient returns a new hyperliquid Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	url, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	return &Client{
		Url:        url,
		HttpClient: cfg.DefaultHttpClient(),
		Asset:      cfgI,
	}, nil
}

// FetchTransferInput returns tx input for a hyperliquid tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	return &tx_input.TxInput{}, errors.New("not implemented")
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a hyperliquid tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	return errors.New("not implemented")
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, errors.New("not implemented")
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	return xclient.TxInfo{}, errors.New("not implemented")
}

func getBlockchainAmount(balance types.SpotBalance, decimals int32) (xc.AmountBlockchain, error) {
	held, err := xc.NewAmountHumanReadableFromStr(balance.Hold)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to convert held amount: %w", err)
	}

	total, err := xc.NewAmountHumanReadableFromStr(balance.Total)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to convert total amount: %w", err)
	}

	bHeld := held.ToBlockchain(decimals)
	bTotal := total.ToBlockchain(decimals)

	return bTotal.Sub(&bHeld), nil
}

func (client *Client) fetchTokensMetadata(ctx context.Context) (types.SpotMetaResponse, error) {
	var tokensMeta types.SpotMetaResponse
	err := client.Call(ctx, "spotMeta", nil, &tokensMeta)
	return tokensMeta, err
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	var spotBalances types.SpotClearinghouseState
	err := client.Call(ctx, "spotClearinghouseState", map[string]any{
		"user": args.Address(),
	}, &spotBalances)

	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to call hyperliquid api: %w", err)
	}

	decimals := HypeDecimals
	name := Hype
	contract, ok := args.Contract()
	if ok {
		tokensMetadata, err := client.fetchTokensMetadata(ctx)
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch tokens metadata: %w", err)
		}
		tokenMeta, ok := tokensMetadata.GetTokenMetaByContract(contract)
		if !ok {
			return xc.AmountBlockchain{}, fmt.Errorf("missing token metadata for contract: %s", contract)
		}

		decimals = tokenMeta.WeiDecimals
		name = tokenMeta.Name
	}

	for _, balance := range spotBalances.Balances {
		if balance.Coin == name {
			amount, err := getBlockchainAmount(balance, int32(decimals))
			return amount, err
		}
	}

	return xc.NewAmountBlockchainFromUint64(0), nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	tokensMeta, err := client.fetchTokensMetadata(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch token metadata: %w")
	}

	if contract == "" {
		contract = HypeContract
	}

	tm, ok := tokensMeta.GetTokenMetaByContract(contract)
	if !ok {
		return 0, fmt.Errorf("missing token metadata for %s", contract)
	}

	return tm.WeiDecimals, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	return &xclient.BlockWithTransactions{}, errors.New("not implemented")
}

func (client *Client) Call(ctx context.Context, method string, params map[string]any, result any) error {
	if params == nil {
		params = make(map[string]any)
	}

	params["type"] = method
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	log := logrus.WithFields(logrus.Fields{
		"url":    client.Url,
		"method": method,
	})
	log.WithField("params", params).Debug("post request")

	req, err := http.NewRequestWithContext(ctx, "POST", client.Url.String(), bytes.NewReader(body))
	if err != nil {
		fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed http.Do: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.WithFields(logrus.Fields{
		"body":   string(body),
		"status": resp.Status,
	}).Debug("got response")

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to unmarshal body: %w", err)
	}

	return nil
}
