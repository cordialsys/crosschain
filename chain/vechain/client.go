package vechain

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm"
	xclient "github.com/cordialsys/crosschain/client"
	clienterrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/cordialsys/crosschain/client/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Client struct {
	Asset      *xc.ChainConfig
	httpClient *http.Client
}

var _ xclient.Client = &Client{}

func NewClient(cfg *xc.ChainConfig) (*Client, error) {
	endpoint, err := url.Parse(cfg.URL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, fmt.Errorf("VeChain requires a Thor HTTP REST URL")
	}
	return &Client{Asset: cfg, httpClient: cfg.DefaultHttpClient()}, nil
}

func (client *Client) request(ctx context.Context, method, path string, body, result any) error {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(client.Asset.URL, "/")+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Thor HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(result)
}

type block struct {
	Number        uint64   `json:"number"`
	ID            string   `json:"id"`
	Timestamp     int64    `json:"timestamp"`
	BaseFeePerGas string   `json:"baseFeePerGas"`
	Transactions  []string `json:"transactions"`
}

func (client *Client) getBlock(ctx context.Context, revision string) (*block, error) {
	var result *block
	if err := client.request(ctx, http.MethodGet, "/blocks/"+url.PathEscape(revision), nil, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, clienterrors.Errorf(clienterrors.TransactionNotFound, "VeChain block %s not found", revision)
	}
	return result, nil
}

type callResult struct {
	Data     string `json:"data"`
	GasUsed  uint64 `json:"gasUsed"`
	Reverted bool   `json:"reverted"`
	VMError  string `json:"vmError"`
}

func (client *Client) simulate(ctx context.Context, c clause, caller xc.Address, revision string) (callResult, error) {
	body := map[string]any{
		"clauses": []any{map[string]any{"to": strings.ToLower(c.To.Hex()), "value": hexutil.EncodeBig(c.Value), "data": hexutil.Encode(c.Data)}},
	}
	if caller != "" {
		body["caller"] = strings.ToLower(string(caller))
	}
	var results []callResult
	if err := client.request(ctx, http.MethodPost, "/accounts/*?revision="+url.QueryEscape(revision), body, &results); err != nil {
		return callResult{}, err
	}
	if len(results) != 1 {
		return callResult{}, fmt.Errorf("Thor simulation returned %d results, expected one", len(results))
	}
	if results[0].Reverted || results[0].VMError != "" {
		return callResult{}, fmt.Errorf("Thor simulation reverted: %s", results[0].VMError)
	}
	return results[0], nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	c, err := transferClause(client.Asset.Base(), args)
	if err != nil {
		return nil, err
	}
	if len(args.GetTransactionAttempts()) > 0 {
		return nil, fmt.Errorf("VeChain retries require reusing the original signed transaction to avoid double sends")
	}
	genesis, err := client.getBlock(ctx, "0")
	if err != nil {
		return nil, err
	}
	genesisID, err := hexutil.Decode(genesis.ID)
	if err != nil || len(genesisID) != 32 {
		return nil, fmt.Errorf("invalid Thor genesis block ID")
	}
	if configured := string(client.Asset.ChainID); configured != "" && !strings.EqualFold(configured, genesis.ID) {
		return nil, fmt.Errorf("Thor genesis block does not match configured VeChain network")
	}
	head, err := client.getBlock(ctx, "best")
	if err != nil {
		return nil, err
	}
	headID, err := hexutil.Decode(head.ID)
	if err != nil || len(headID) != 32 {
		return nil, fmt.Errorf("invalid Thor block ID")
	}
	base, err := hexutil.DecodeBig(head.BaseFeePerGas)
	if err != nil {
		return nil, fmt.Errorf("invalid Thor base fee: %w", err)
	}
	var priority struct {
		Fee string `json:"maxPriorityFeePerGas"`
	}
	if err := client.request(ctx, http.MethodGet, "/fees/priority", nil, &priority); err != nil {
		return nil, err
	}
	tip, err := hexutil.DecodeBig(priority.Fee)
	if err != nil {
		return nil, fmt.Errorf("invalid Thor priority fee: %w", err)
	}
	result, err := client.simulate(ctx, c, args.GetFrom(), "next")
	if err != nil {
		return nil, err
	}
	intrinsic := intrinsicGas(c)
	if result.GasUsed > math.MaxUint64-intrinsic-1000 {
		return nil, fmt.Errorf("Thor gas estimate overflow")
	}
	input := NewTxInput()
	input.ChainTag = genesisID[31]
	input.BlockRef = binary.BigEndian.Uint64(headID[:8])
	input.FromAddress = args.GetFrom()
	input.GasLimit = intrinsic + result.GasUsed
	if len(c.Data) > 0 {
		input.GasLimit += 1000
	}
	input.GasTipCap = xc.AmountBlockchain(*tip).ApplyGasPriceMultiplier(client.Asset.Client())
	baseFee := xc.AmountBlockchain(*base)
	maxFee := baseFee.Add(&baseFee)
	maxFee = maxFee.Add(&input.GasTipCap)
	input.GasFeeCap = maxFee.ApplyGasPriceMultiplier(client.Asset.Client())
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	input.Nonce = binary.BigEndian.Uint64(nonce[:])
	return input, nil
}

func (client *Client) SubmitTx(ctx context.Context, req types.SubmitTxReq, _ xcbuilder.SubmitArgs) error {
	return client.request(ctx, http.MethodPost, "/transactions", map[string]string{"raw": hexutil.Encode(req.TxData)}, nil)
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	if err := evm.ValidateAddress(client.Asset.Base(), args.Address()); err != nil {
		return zero, err
	}
	if contract, ok := args.Contract(); ok {
		data := append([]byte{0x70, 0xa0, 0x82, 0x31}, make([]byte, 12)...)
		address, _ := hexutil.Decode(string(args.Address()))
		data = append(data, address...)
		return client.tokenCall(ctx, contract, data)
	}
	var result struct {
		Balance string `json:"balance"`
	}
	if err := client.request(ctx, http.MethodGet, "/accounts/"+string(args.Address()), nil, &result); err != nil {
		return zero, err
	}
	return parseAmount(result.Balance)
}

func (client *Client) tokenCall(ctx context.Context, contract xc.ContractAddress, data []byte) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	if err := evm.ValidateAddress(client.Asset.Base(), xc.Address(contract)); err != nil {
		return zero, err
	}
	address, err := hexutil.Decode(string(contract))
	if err != nil {
		return zero, err
	}
	c := clause{Value: zero.Int(), Data: data}
	copy(c.To[:], address)
	result, err := client.simulate(ctx, c, "", "best")
	if err != nil {
		return zero, err
	}
	bytes, err := hexutil.Decode(result.Data)
	if err != nil || len(bytes) != 32 {
		return zero, fmt.Errorf("invalid Thor token call result")
	}
	return xc.AmountBlockchain(*new(big.Int).SetBytes(bytes)), nil
}

func parseAmount(value string) (xc.AmountBlockchain, error) {
	number, err := hexutil.DecodeBig(value)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}
	return xc.AmountBlockchain(*number), nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.IsChain(contract) {
		return int(client.Asset.Decimals), nil
	}
	amount, err := client.tokenCall(ctx, contract, []byte{0x31, 0x3c, 0xe5, 0x67})
	if err != nil {
		return 0, err
	}
	if !amount.Int().IsUint64() || amount.Uint64() > 255 {
		return 0, fmt.Errorf("invalid token decimals")
	}
	return int(amount.Uint64()), nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	revision := "best"
	if height, ok := args.Height(); ok {
		revision = strconv.FormatUint(height, 10)
	}
	result, err := client.getBlock(ctx, revision)
	if err != nil {
		return nil, err
	}
	return &txinfo.BlockWithTransactions{Block: *txinfo.NewBlock(client.Asset.Chain, result.Number, result.ID, time.Unix(result.Timestamp, 0)), TransactionIds: result.Transactions}, nil
}
