package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/client/types"
	"github.com/cordialsys/crosschain/chain/hyperliquid/client/wstypes"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	EndpointExchange                  = "exchange"
	EndpointExplorer                  = "explorer"
	EndpointInfo                      = "info"
	Hype                              = "HYPE"
	HypeContractMainnet               = xc.ContractAddress("HYPE:0x0d01dc56dcaaca66ad901c959b4011ec")
	HypeContractTestnet               = xc.ContractAddress("HYPE:0x7317beb7cceed72ef0b346074cc8e7ab")
	HypeDecimals                      = 8
	MethodBlockDetails                = "blockDetails"
	MethodSpotClearingHouseState      = "spotClearinghouseState"
	MethodSpotMeta                    = "spotMeta"
	MethodTxDetails                   = "txDetails"
	MethodUserDetails                 = "userDetails"
	MethodUserNonFundingLedgerUpdates = "userNonFundingLedgerUpdates"
	ResponseTypeError                 = "error"
	WebsocketUrlMainnet               = "wss://api.hyperliquid.xyz/ws"
	WebsocketUrlTestnet               = "wss://api.hyperliquid-testnet.xyz/ws"
)

// Client for hyperliquid
// Hyperliquid is relying on two APIs: "explorer" and "info"
// - "info" api is main hyperliquid api
// - "explorer" api is available only via RPC, it provides transaction/user/block details
type Client struct {
	Asset            xc.ITask
	ApiUrl           *url.URL
	RpcUrl           *url.URL
	HypeContract     xc.ContractAddress
	HyperliquidChain string
	HttpClient       *http.Client
	WebsocketUrl     *url.URL
}

var _ xclient.Client = &Client{}

// NewClient returns a new hyperliquid Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()

	var hyperliquidChain string
	var rpcUrl *url.URL
	var err error
	var hypeContract xc.ContractAddress
	var wssUrl *url.URL
	if cfg.Network == "mainnet" {
		rpcUrl, err = url.Parse("https://rpc.hyperliquid.xyz")
		if err != nil {
			return nil, fmt.Errorf("failed to parse mainnet rpc url: %w", err)
		}
		wssUrl, err = url.Parse(WebsocketUrlMainnet)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mainnet wss url: %w", err)
		}
		hyperliquidChain = "Mainnet"
		hypeContract = HypeContractMainnet
	} else {
		rpcUrl, err = url.Parse("https://rpc.hyperliquid-testnet.xyz")
		if err != nil {
			return nil, fmt.Errorf("failed to parse testnet rpc url: %w", err)
		}

		wssUrl, err = url.Parse(WebsocketUrlTestnet)
		if err != nil {
			return nil, fmt.Errorf("failed to parse testnet wss url: %w", err)
		}
		hyperliquidChain = "Testnet"
		hypeContract = HypeContractTestnet
	}

	url, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	return &Client{
		ApiUrl:           url,
		RpcUrl:           rpcUrl,
		HypeContract:     hypeContract,
		HyperliquidChain: hyperliquidChain,
		HttpClient:       cfg.DefaultHttpClient(),
		Asset:            cfgI,
		WebsocketUrl:     wssUrl,
	}, nil
}

// FetchTransferInput returns tx input for a hyperliquid tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput := tx_input.NewTxInput()
	txInput.TransactionTime = time.Now()

	contract, ok := args.GetContract()
	if !ok {
		contract = client.HypeContract
	}
	txInput.Token = contract

	decimals, err := client.FetchDecimals(ctx, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch decimals: %w", err)
	}
	txInput.Decimals = int32(decimals)
	txInput.HyperliquidChain = client.HyperliquidChain

	return txInput, nil
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
	payload, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	response, err := client.CallExchange(ctx, payload)
	if err != nil {
		if apiErr, ok := err.(types.APIError); ok {
			if strings.Contains(apiErr.Response, "duplicate nonce") {
				return xcerrors.TransactionExistsf("%v", err)
			}
		}
		return fmt.Errorf("failed to post transaction: %w", err)
	}

	if !response.IsOk() {
		return fmt.Errorf("failed to submit tx: %s", response.Response)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, errors.New("not implemented")
}

func (client *Client) fetchTxDetails(ctx context.Context, hash string) (types.Transaction, error) {
	type response struct {
		Type        string            `json:"type"`
		Transaction types.Transaction `json:"tx"`
		// Error message
		Message string `json:"message"`
	}

	var txDetails response
	err := client.CallExplorer(ctx, MethodTxDetails, map[string]any{
		"hash": hash,
	}, &txDetails)

	if txDetails.Type == ResponseTypeError {
		return types.Transaction{}, xcerrors.TransactionNotFoundf("%s", txDetails.Message)
	}

	return txDetails.Transaction, err

}

func (client *Client) fetchTransactionFee(ctx context.Context, address xc.Address, hash xc.TxHash) (xc.AmountHumanReadable, string, error) {
	var response []types.UserNonFundingLedgerUpdate
	err := client.CallInfo(ctx, MethodUserNonFundingLedgerUpdates, map[string]any{
		"user": address,
	}, &response)
	if err != nil {
		return xc.AmountHumanReadable{}, "", fmt.Errorf("failed to fetch user fees: %w", err)
	}

	for _, update := range response {
		if update.Hash == string(hash) {
			fee := update.GetFee()
			feeHr, err := xc.NewAmountHumanReadableFromStr(fee)
			if err != nil {
				return xc.AmountHumanReadable{}, "", fmt.Errorf("failed to parse amount human readable: %w", err)
			}

			token := update.GetFeeToken()

			return feeHr, token, nil
		}
	}

	return xc.AmountHumanReadable{}, "", fmt.Errorf("coudln't find tx %s in user ledger updates", hash)
}

// Traditional "FetchTxInfo" implementation - use a native Hyperliquid transaction hash for transaction
// info lookup
func (client *Client) fetchTxInfoByHash(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	contract, _ := args.Contract()
	txDetails, err := client.fetchTxDetails(ctx, string(args.TxHash()))
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch tx details: %w", err)
	}

	spotSend, ok, err := txDetails.GetSpotSend()
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get spotSend action: %w", err)
	}
	if !ok {
		return xclient.TxInfo{}, errors.New("tx-info supports only spotSend actions for now")
	}

	chain := client.Asset.GetChain().Chain
	blockTime := time.UnixMilli(txDetails.Time)
	block := xclient.NewBlock(chain, txDetails.Block, txDetails.Hash, blockTime)
	latestHeight, err := client.fetchBlockHeight(ctx)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch latest height: %w", err)
	}
	confirmations := uint64(0)
	if latestHeight > block.Height.Uint64() {
		confirmations = latestHeight - block.Height.Uint64()
	}

	txInfo := xclient.NewTxInfo(block, client.Asset.GetChain(), txDetails.Hash, confirmations, &txDetails.Error)
	sourceAddress := xc.Address(txDetails.User)
	destinationAddress := xc.Address(spotSend.Destination)
	hrAmount, err := xc.NewAmountHumanReadableFromStr(spotSend.Amount)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to convert amount to HumanReadable: %w", err)
	}
	decimals, err := client.FetchDecimals(ctx, contract)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch token decimals: %w", err)
	}
	amount := hrAmount.ToBlockchain(int32(decimals))
	movement := xclient.NewMovement(chain, contract)
	movement.AddSource(sourceAddress, amount, nil)
	movement.AddDestination(destinationAddress, amount, nil)
	txInfo.AddMovement(movement)

	fee, feeToken, err := client.fetchTransactionFee(ctx, sourceAddress, args.TxHash())
	tokensMetadata, err := client.fetchTokensMetadata(ctx)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch tokens metadata: %w", err)
	}

	feeDecimals := HypeDecimals
	feeContract := xc.ContractAddress("")
	tokenMeta, ok := tokensMetadata.GetTokenMetaByName(feeToken)
	if ok {
		feeDecimals = tokenMeta.WeiDecimals
		feeContract = xc.ContractAddress(tokenMeta.TokenId)
	}

	feeAmount := fee.ToBlockchain(int32(feeDecimals))
	txInfo.AddFee(sourceAddress, xc.ContractAddress(feeContract), feeAmount, nil)
	txInfo.Fees = txInfo.CalculateFees()
	txInfo.Final = int(txInfo.Confirmations) > client.Asset.GetChain().ConfirmationsFinal

	return *txInfo, nil
}

// ActionHash + SenderAddres transaction info lookup. It relies on "fetchTxInfoByHash" under the hood.
// Required because native hyperliquid transaction hash contains block related data, so we cannot calculate
// it upfront.
func (client *Client) fetchTxInfoByActionHash(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	sender, ok := args.Sender()
	if !ok {
		return xclient.TxInfo{}, fmt.Errorf("missing sender address")
	}

	var userDetails types.UserDetails
	err := client.CallExplorer(ctx, MethodUserDetails, map[string]any{
		"user": string(sender),
	}, &userDetails)

	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch user details: %w", err)
	}

	for _, tx := range userDetails.Txs {
		spotSend, ok, err := tx.GetSpotSend()
		if err != nil {
			return xclient.TxInfo{}, fmt.Errorf("failed to get tx action: %w", err)
		}

		if ok {
			ah, err := types.GetActionHash(spotSend)
			if err != nil {
				return xclient.TxInfo{}, fmt.Errorf("failed to calculate action hash for tx %s: %w", tx.Hash, err)
			}

			fmt.Printf("Action: %s, hash: %s, spotSend: %+v\n", ah, tx.Hash, spotSend)
			if ah == string(args.TxHash()) {
				args.SetHash(xc.TxHash(tx.Hash))
				return client.fetchTxInfoByHash(ctx, args)
			}
		}
	}

	return xclient.TxInfo{}, xcerrors.TransactionNotFoundf("%v", err)
}

// Fetch transaction info
// - Fetch by Hyperliquid tx hash if no 'Sender' is provided
// - Fetch by Hyperliquid action hash if 'Sender' is provided
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	hash := args.TxHash()
	if !strings.HasPrefix(string(hash), "0x") {
		// normalize the prefix if valid hex
		_, err := hex.DecodeString(string(hash))
		if err != nil {
			return xclient.TxInfo{}, fmt.Errorf("failed to decode hash as hex: %w", err)
		}
		hash = "0x" + hash
		args.SetHash(xc.TxHash(hash))
	}

	_, ok := args.Sender()
	if ok {
		return client.fetchTxInfoByActionHash(ctx, args)
	} else {
		return client.fetchTxInfoByHash(ctx, args)
	}
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

// Fetch a list of supported hype tokens
func (client *Client) fetchTokensMetadata(ctx context.Context) (types.SpotMetaResponse, error) {
	var tokensMeta types.SpotMetaResponse
	err := client.CallInfo(ctx, MethodSpotMeta, nil, &tokensMeta)
	return tokensMeta, err
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	var spotBalances types.SpotClearinghouseState
	err := client.CallInfo(ctx, MethodSpotClearingHouseState, map[string]any{
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
		n, _, ok := strings.Cut(string(contract), ":")
		if !ok {
			return xc.AmountBlockchain{}, fmt.Errorf("invalid contract format, expected 'Name:TokenId', got: %s", contract)
		}
		tokenMeta, ok := tokensMetadata.GetTokenMetaByName(n)
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
		return 0, fmt.Errorf("failed to fetch token metadata: %w", err)
	}

	if contract == "" {
		contract = client.HypeContract
	}

	name, _, ok := strings.Cut(string(contract), ":")
	if !ok {
		return 0, fmt.Errorf("invalid contract format, expected 'Name:TokenId', got: %s", contract)
	}

	tm, ok := tokensMeta.GetTokenMetaByName(name)
	if !ok {
		return 0, fmt.Errorf("missing token metadata for %s", contract)
	}

	return tm.WeiDecimals, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, ok := args.Height()
	if !ok {
		h, err := client.fetchBlockHeight(ctx)
		if err != nil {
			return nil, err
		}
		height = h
	}

	type response struct {
		Type         string             `json:"type"`
		BlockDetails types.BlockDetails `json:"blockDetails"`
	}
	var resp response
	err := client.CallExplorer(ctx, MethodBlockDetails, map[string]any{
		"height": height,
	}, &resp)

	if err != nil {
		return nil, err
	}

	transactions := make([]string, resp.BlockDetails.NumTxs)
	for i, tx := range resp.BlockDetails.Txs {
		transactions[i] = tx.Hash
	}

	timestamp := time.UnixMilli(resp.BlockDetails.BlockTime)
	block := xclient.NewBlock(client.Asset.GetChain().Chain, height, resp.BlockDetails.Hash, timestamp)
	return &xclient.BlockWithTransactions{
		Block:          *block,
		TransactionIds: transactions,
		SubBlocks:      []*xclient.SubBlockWithTransactions{},
	}, nil
}

func (client *Client) callInner(ctx context.Context, url string, method string, params map[string]any, result any) error {
	if params == nil {
		params = make(map[string]any)
	}

	params["type"] = method
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	log := logrus.WithFields(logrus.Fields{
		"url":    url,
		"method": method,
	})
	log.WithField("params", params).Debug("post request")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
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

func (c *Client) CallInfo(ctx context.Context, method string, params map[string]any, result any) error {
	url := fmt.Sprintf("%s/%s", c.ApiUrl, EndpointInfo)
	return c.callInner(ctx, url, method, params, result)
}

func (c *Client) CallExplorer(ctx context.Context, method string, params map[string]any, result any) error {
	url := fmt.Sprintf("%s/%s", c.RpcUrl, EndpointExplorer)
	return c.callInner(ctx, url, method, params, result)
}

func (c *Client) CallExchange(ctx context.Context, payload []byte) (types.APIResponse, error) {
	url := fmt.Sprintf("%s/%s", c.ApiUrl, EndpointExchange)
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		url,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return types.APIResponse{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	log := logrus.WithFields(logrus.Fields{
		"url":    url,
		"method": "POST",
		"body":   string(payload),
	})
	log.Debug("request")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return types.APIResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body := []byte{}
	if resp.Body != nil {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return types.APIResponse{}, fmt.Errorf("failed to read response body: %w", err)
		}
	}
	log.WithField("body", string(body)).WithField("status", resp.StatusCode).Debug("response")

	if resp.StatusCode != http.StatusOK {
		var e types.APIError
		if err := json.Unmarshal(body, &e); err != nil {
			return types.APIResponse{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
		}
		return types.APIResponse{}, e
	} else {
		var r types.APIResponse
		if err := json.Unmarshal(body, &r); err != nil {
			return types.APIResponse{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
		}
		// unfortunately the API node will return errors on 200 status code
		if r.Status == "err" {
			var e types.APIError
			if err := json.Unmarshal(body, &e); err != nil {
				return types.APIResponse{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			}
			return types.APIResponse{}, e
		}

		return r, nil
	}
}

// There is no easy way to fetch blockHeight on hyperliquid l1.
// As a workaround we use a websocket stream, where we subscribe to ongoing transactions.
// With this info we are able to fetch block height via 'txDetails' explorer API call.
func (client *Client) fetchBlockHeight(ctx context.Context) (uint64, error) {
	wssUrl := client.WebsocketUrl.String()
	c, _, err := websocket.DefaultDialer.Dial(wssUrl, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to connect websocket: %w", err)
	}
	defer c.Close()

	subscription := wstypes.CoinSubscription{
		Type: "trades",
		Coin: Hype,
	}
	request := wstypes.Request{
		Method:       "subscribe",
		Subscription: &subscription,
	}

	logger := logrus.WithField("url", wssUrl)
	payload, err := json.Marshal(request)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal subscription payload: %w", err)
	}

	logger.Info("subscribing to hype trades")
	err = c.WriteMessage(websocket.TextMessage, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write ws message: %w", err)
	}

	// Read subscription response
	_, m, err := c.ReadMessage()
	if err != nil {
		return 0, fmt.Errorf("failed to read subscription reply message: %w", err)
	}

	var response wstypes.Message[wstypes.Request]
	err = json.Unmarshal(m, &response)
	if err != nil {
		return 0, fmt.Errorf("failed to read ws subscription response: %w", err)
	}

	if response.Data.Subscription.Coin != request.Subscription.Coin {
		return 0, fmt.Errorf(
			"subscription mismatch, expected %s got %s",
			request.Subscription.Coin,
			response.Data.Subscription.Coin,
		)
	}

	logger.WithField("response", response.Data).Debug("got subscription response")

	// Read latest 'HYPE' trade
	_, m, err = c.ReadMessage()
	if err != nil {
		return 0, fmt.Errorf("failed to read trades message: %w", err)
	}
	var trades wstypes.Message[[]wstypes.Trade]
	err = json.Unmarshal(m, &trades)
	if err != nil {
		return 0, fmt.Errorf("failed to read ws trades response: %w", err)
	}

	// Don't log 'trades.Data', it's too long even for Debug output
	logger.WithField("trade_count", len(trades.Data)).Debug("got subscription response")
	if len(trades.Data) == 0 {
		return 0, fmt.Errorf("empty block")
	}

	// Sometimes hashless actions are reported over this subscription, we can safely ignroe
	// transactions with empty hashes.
	// TODO: It's unlikely that the block will contain only 'defaultHash' transactions,
	// but consider wrapping this logic in a retry loop
	defaultHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	for _, tx := range trades.Data {
		if tx.Hash != defaultHash {
			txDetails, err := client.fetchTxDetails(ctx, tx.Hash)
			if err != nil {
				return 0, fmt.Errorf("failed to fetch tx-info: %w", err)
			}

			return txDetails.Block, nil
		}
	}

	return 0, fmt.Errorf("failed to fetch block height")
}
