package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/tron/core"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
)

var _ xclient.Client = &Client{}

const TRANSFER_EVENT_HASH_HEX = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const TX_TIMEOUT = 2 * time.Hour

// Client for Template
type Client struct {
	// client *client.GrpcClient
	client *httpclient.Client

	chain *xc.ChainConfig
}

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope

	// 6th to 8th (exclusive) byte of the reference block height
	RefBlockBytes []byte `json:"ref_block_bytes,omitempty"`
	// 8th to 16th (exclusive) byte of the reference block hash
	RefBlockHash []byte `json:"ref_block_hash,omitempty"`

	// Expiration time (seconds)
	Expiration int64 `json:"expiration,omitempty"`
	// Transaction creation time (seconds)
	Timestamp int64 `json:"timestamp,omitempty"`
	// Max fee budget
	MaxFee xc.AmountBlockchain `json:"max_fee,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverTron,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverTron
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	// tron doesn't do prioritization
	_ = multiplier
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return input.MaxFee, ""
}

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
	input.Expiration = unix + int64((TX_TIMEOUT).Seconds())
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// tron uses recent-block-hash like mechanism like solana, but with explicit timestamps
	return true
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	oldInput, ok := other.(*TxInput)
	if ok {
		if input.Timestamp <= oldInput.Expiration {
			return false
		}
	} else {
		// can't tell (this shouldn't happen) - default false
		return false
	}
	// all others timed out - we're safe
	return true
}

func (input *TxInput) ToRawData(contract *core.Transaction_Contract) *core.TransactionRaw {
	return &core.TransactionRaw{
		Contract:      []*core.Transaction_Contract{contract},
		RefBlockBytes: input.RefBlockBytes,
		RefBlockHash:  input.RefBlockHash,
		// tron wants milliseconds
		Expiration: time.Unix(input.Expiration, 0).UnixMilli(),
		Timestamp:  time.Unix(input.Timestamp, 0).UnixMilli(),

		// unused ?
		RefBlockNum: 0,
	}
}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()

	if cfg.GasBudgetDefault.Decimal().InexactFloat64() <= 0 {
		return nil, fmt.Errorf("chain gas-budget-default should be set to value greater than 0.0")
	}

	client, err := httpclient.NewHttpClient(cfg.URL, cfg.DefaultHttpClient().Timeout)
	if err != nil {
		return nil, err
	}

	return &Client{
		client,
		cfg,
	}, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := new(TxInput)

	// Getting blockhash details from the CreateTransfer endpoint as TRON uses an unusual hashing algorithm (SHA2256SM3), so we can't do a minimal
	// retrieval and just get the blockheaders
	dummyTx, err := client.client.CreateTransaction(string(args.GetFrom()), string(args.GetTo()), 1)
	if err != nil {
		return nil, err
	}

	input.RefBlockBytes = dummyTx.RawData.RefBlockBytes
	input.RefBlockHash = dummyTx.RawData.RefBlockHashBytes
	// set timeout period
	input.Timestamp = time.Now().Unix()
	input.Expiration = time.Now().Add(TX_TIMEOUT).Unix()

	maxFee := client.chain.GasBudgetDefault.ToBlockchain(client.chain.Decimals)
	input.MaxFee = maxFee

	return input, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.chain.Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Tron tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	bz, err := tx.Serialize()
	if err != nil {
		return err
	}

	_, err = client.client.BroadcastHex(hex.EncodeToString(bz))

	return err
}

// Some data is available via the EVM RPC methods, but not the native methods :/
func (client *Client) ConnectEvmJsonRpc(ctx context.Context) (*ethclient.Client, error) {
	jsonRpcUrl, err := url.Parse(client.chain.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %v", err)
	}

	jsonRpcUrl = jsonRpcUrl.JoinPath("jsonrpc")
	jsonRpcSocket, err := rpc.DialHTTPWithClient(jsonRpcUrl.String(), client.client.HttpClient())
	if err != nil {
		return nil, fmt.Errorf("dialing url: %v", client.chain.URL)
	}
	rpcClient := ethclient.NewClient(jsonRpcSocket)

	return rpcClient, nil
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	rpcClient, err := client.ConnectEvmJsonRpc(ctx)
	if err != nil {
		return xclient.LegacyTxInfo{}, err
	}
	tx, err := client.client.GetTransactionByID(string(txHash))
	if err != nil {
		return xclient.LegacyTxInfo{}, err
	}

	info, err := client.client.GetTransactionInfoByID(string(txHash))
	if err != nil {
		return xclient.LegacyTxInfo{}, err
	}
	evmTx, err := rpcClient.TransactionReceipt(ctx, common.HexToHash(string(txHash)))
	if err != nil {
		return xclient.LegacyTxInfo{}, fmt.Errorf("failed to get transaction by hash from EVM RPC: %v", err)
	}

	latestBlockResp, err := client.client.GetBlockByLatest(1)
	if err != nil {
		return xclient.LegacyTxInfo{}, err
	}
	latestBlock := latestBlockResp.Block[0]

	var from xc.Address
	var to xc.Address
	var amount xc.AmountBlockchain
	sources, destinations := deserialiseTransactionEvents(info.Logs, evmTx)
	// If we cannot retrieve transaction events, we can infer that the TX is a native transfer
	if len(sources) == 0 && len(destinations) == 0 {
		from, to, amount, err = deserialiseNativeTransfer(tx)
		if err != nil {
			logrus.WithError(err).Warn("unknown transaction")
		} else {
			source := new(xclient.LegacyTxInfoEndpoint)
			source.Address = from
			source.Amount = amount
			source.Asset = string(client.chain.Chain)
			source.NativeAsset = client.chain.Chain

			destination := new(xclient.LegacyTxInfoEndpoint)
			destination.Address = to
			destination.Amount = amount
			destination.Asset = string(client.chain.Chain)
			destination.NativeAsset = client.chain.Chain
			destination.Event = xclient.NewEvent("", xclient.MovementVariantNative)

			sources = append(sources, source)
			destinations = append(destinations, destination)
		}
	}

	blockHash := evmTx.BlockHash.String()
	// Tron natively doesn't use 0x prefix.
	blockHash = strings.TrimPrefix(blockHash, "0x")

	txInfo := xclient.LegacyTxInfo{
		BlockHash:       blockHash,
		TxID:            string(txHash),
		From:            from,
		To:              to,
		ContractAddress: xc.ContractAddress(info.ContractAddress),
		Amount:          amount,
		Fee:             xc.NewAmountBlockchainFromUint64(uint64(info.Fee)),
		BlockIndex:      int64(info.BlockNumber),
		BlockTime:       int64(info.BlockTimeStamp / 1000),
		Confirmations:   int64(latestBlock.BlockHeader.RawData.Number - info.BlockNumber),
		Status:          xc.TxStatusSuccess,
		Sources:         sources,
		Destinations:    destinations,
		Time:            int64(info.BlockTimeStamp),
		TimeReceived:    0,
		Error:           "",
	}
	switch info.Receipt.Result {
	// Tron is not very consistent in how it reports success
	case httpclient.Success, "", "success":
		txInfo.Status = xc.TxStatusSuccess
	default:
		// assume that it reverted
		txInfo.Error = string(info.Receipt.Result)
		txInfo.Sources = nil
		txInfo.Destinations = nil
	}

	return txInfo, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, args.TxHash())
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(client.chain, legacyTx, xclient.Account), nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	if contract, ok := args.Contract(); ok {
		a, err := client.client.ReadTrc20Balance(string(args.Address()), string(contract))
		if err != nil {
			return xc.AmountBlockchain{}, err
		}
		return xc.NewAmountBlockchainFromStr(a.String()), nil
	} else {
		return client.FetchNativeBalance(ctx, args.Address())
	}
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	resp, err := client.client.GetAccount(string(address))
	if err != nil {
		return xc.AmountBlockchain{}, err
	}

	return xc.NewAmountBlockchainFromUint64(uint64(resp.Balance)), nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.chain.IsChain(contract) {
		return int(client.chain.Decimals), nil
	}

	dec, err := client.client.ReadTrc20Decimals(string(contract))
	if err != nil {
		return 0, err
	}
	return int(dec.Uint64()), nil
}
func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	var tronBlock *httpclient.BlockResponse
	height, ok := args.Height()
	if !ok {
		response, err := client.client.GetBlockByLatest(1)
		if err != nil {
			return nil, err
		}
		if len(response.Block) == 0 {
			return nil, fmt.Errorf("no blocks found on chain")
		}
		tronBlock = response.Block[0]
	} else {
		response, err := client.client.GetBlockByNum(height)
		if err != nil {
			return nil, err
		}
		tronBlock = response
	}
	height = tronBlock.BlockHeader.RawData.Number

	txs := tronBlock.Transactions

	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(
			client.chain.Chain,
			height,
			tronBlock.BlockId,
			time.Unix(int64(tronBlock.BlockHeader.RawData.Timestamp/1000), 0),
		),
	}
	for _, tx := range txs {
		block.TransactionIds = append(block.TransactionIds, tx.TxID)
	}

	return block, nil
}
