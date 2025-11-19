package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	resttypes "github.com/cordialsys/crosschain/chain/hedera/client/rest_types"
	commontypes "github.com/cordialsys/crosschain/chain/hedera/common_types"
	"github.com/cordialsys/crosschain/chain/hedera/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	clienterrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/cordialsys/hedera-protobufs-go/services"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	API_VERSION = "api/v1"
	// Max validity time for hedera transactions is 180 seconds
	DEFAULT_VALIDITY_TIME   = int64(180)
	ENDPOINT_ACCOUNTS       = "accounts"
	ENDPOINT_BLOCKS         = "blocks"
	ENDPOINT_EXCHAINGE_RATE = "network/exchangerate"
	ENDPOINT_TOKENS         = "tokens"
	ENDPOINT_TRANSACTIONS   = "transactions"
	HBAR                    = "HBAR"
	KEY_LIMIT               = "limit"
	KEY_ORDER               = "order"
	MAX_PAGES               = 50
	ORDER_ASC               = "asc"
	RESULT_SUCCESS          = "SUCCESS"
)

// Cost of CRYPTO_TRANSFER operation in USD's
// https://docs.hedera.com/hedera/networks/mainnet/fees
// Use mirror `api/v1/network/exchangerate` to convert to HBAR
var CRYPTO_TRANSFER_FEE = xc.MustNewAmountHumanReadableFromStr("0.0001")

// Fees go to fee accounts + node operator
// Fee accounts are the same for testnet and mainnet
var FeeAccounts = []string{
	"0.0.98",
	"0.0.800",
	"0.0.801",
}

// Client for Hedera
type Client struct {
	Asset        *xc.ChainConfig
	CryptoClient services.CryptoServiceClient
	HttpClient   *http.Client
	IndexerUrl   *url.URL
	Logger       *logrus.Entry
	NodeId       string
}

var _ xclient.Client = &Client{}

// NewClient returns a new Hedera Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	url, err := url.Parse(cfg.IndexerUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid indexer url: %w", err)
	}

	grpcUrl := cfg.URL
	grpcClient, err := grpc.NewClient(grpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}
	cryptoClient := services.NewCryptoServiceClient(grpcClient)

	if cfg.ChainID == "" {
		return nil, fmt.Errorf("required a proper chain_id (node id) for hedera clients")
	}

	return &Client{
		Asset:        cfg,
		CryptoClient: cryptoClient,
		HttpClient:   http.DefaultClient,
		IndexerUrl:   url.JoinPath(API_VERSION),
		Logger: logrus.WithFields(logrus.Fields{
			"chain": cfg.Chain,
		}),
		NodeId: string(cfg.ChainID),
	}, nil
}

func (c *Client) FetchAccountInfo(ctx context.Context, address xc.Address) (*resttypes.AccountInfo, error) {
	url := c.IndexerUrl.JoinPath(ENDPOINT_ACCOUNTS + "/" + string(address))
	accountInfo, err := Get[resttypes.AccountInfo](ctx, c, url)
	if err != nil {
		return nil, fmt.Errorf("failed to get: %w", err)
	}

	return accountInfo, nil
}

func (c *Client) FetchExchangeRate(ctx context.Context) (*resttypes.ExchangeRate, error) {
	u := c.IndexerUrl.JoinPath(ENDPOINT_EXCHAINGE_RATE)
	return Get[resttypes.ExchangeRate](ctx, c, u)
}

// FetchTransferInput returns tx input for a Hedera tx
func (c *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	evmAddress := args.GetFrom()
	accInfo, err := c.FetchAccountInfo(ctx, evmAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accountInfo: %w", err)
	}

	blockInfo, err := c.FetchRawBlock(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block data: %w", err)
	}

	t, err := blockInfo.GetBlockTime()
	if err != nil {
		return nil, fmt.Errorf("failed to read consensus timestamp: %w", err)
	}
	ts := t.UnixNano()
	memo, _ := args.GetMemo()
	if len(memo) > commontypes.MAX_MEMO_LENGTH {
		return nil, fmt.Errorf("memo is too long(%d), max length: %d", len(memo), commontypes.MAX_MEMO_LENGTH)
	}

	validTime := DEFAULT_VALIDITY_TIME
	if c.Asset.TransactionActiveTime > time.Second {
		validTime = int64(c.Asset.TransactionActiveTime / time.Second)
	}

	rate, err := c.FetchExchangeRate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange rate: %w", err)
	}

	decimals, err := c.FetchDecimals(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch decimals: %w", err)
	}

	fee := rate.GetMaxEquivalent(CRYPTO_TRANSFER_FEE)
	feeMultiplier := c.Asset.ChainGasMultiplier
	hrFeeMultiplier := xc.NewAmountHumanReadableFromFloat(feeMultiplier)
	fee = fee.Mul(hrFeeMultiplier)
	input := &tx_input.TxInput{
		TxInputEnvelope:     xc.TxInputEnvelope{},
		NodeAccountID:       c.NodeId,
		ValidStartTimestamp: ts,
		MaxTransactionFee:   fee.ToBlockchain(int32(decimals)).Uint64(),
		ValidTime:           validTime,
		Memo:                memo,
		AccountId:           accInfo.Account,
	}
	return input, nil
}

// Deprecated method - use FetchTransferInput
func (c *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := c.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return c.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Hedera tx
func (c *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	logger := c.GrpcLogger(ctx)
	txBytes, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}
	reqTx := &services.Transaction{
		SignedTransactionBytes: txBytes,
	}

	logger.WithField("transaction", fmt.Sprintf("%+v", reqTx)).Debug("calling crypto transfer")
	r, err := c.CryptoClient.CryptoTransfer(ctx, reqTx)
	if err != nil {
		return fmt.Errorf("failed transaction submission: %w", err)
	}

	logger.WithField("response", fmt.Sprintf("%+v", r)).Debug("got grpc response")
	// there is also `ResponseCodeEnum_Success` but it's not used
	if r.NodeTransactionPrecheckCode != services.ResponseCodeEnum_OK {
		grpcError := commontypes.GrpcError(r.NodeTransactionPrecheckCode)
		if r.NodeTransactionPrecheckCode == services.ResponseCodeEnum_DUPLICATE_TRANSACTION {
			return clienterrors.TransactionExistsf("node precheck failure: %s", grpcError)
		} else {
			return fmt.Errorf("node precheck failure: %w", grpcError)
		}
	}
	return nil
}

// Returns transaction info - legacy/old endpoint
func (c *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, errors.New("not implemented")
}

func determineEventType(transfer resttypes.Transfer, node string) xclient.MovementVariant {
	if slices.Contains(FeeAccounts, transfer.Account) || transfer.Account == node {
		return xclient.MovementVariantFee
	} else if transfer.TokenId == "" {
		return xclient.MovementVariantNative
	} else {
		return xclient.MovementVariantToken
	}
}

// Returns transaction info - new endpoint
func (c *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {
	u := c.IndexerUrl.JoinPath(ENDPOINT_TRANSACTIONS + "/" + string(args.TxHash()))
	transactions, err := Get[resttypes.TransactionsInfo](ctx, c, u)
	if err != nil && strings.Contains(err.Error(), "Not Found") {
		return xclient.TxInfo{}, clienterrors.TransactionNotFoundf("%s", err.Error())
	} else if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch transaction: %w", err)
	}

	if len(transactions.Transactions) == 0 {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch transaction: %s", args.TxHash())
	}
	// select parent transaction
	var tx *resttypes.Transaction
	for _, t := range transactions.Transactions {
		if t.Nonce == 0 {
			tx = &t
			break
		}
	}

	if tx == nil {
		return xclient.TxInfo{}, errors.New("failed to find transaction with nonce 0")
	}

	pKey, pValue := tx.BlockTimeParam()
	params := url.Values{}
	params.Add(pKey, pValue)
	params.Add(KEY_LIMIT, "1")
	params.Add(KEY_ORDER, ORDER_ASC)
	blocks, err := c.FetchRawBlocks(ctx, params)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch blocks: %w", err)
	}
	if len(blocks.Blocks) != 1 {
		return xclient.TxInfo{}, fmt.Errorf("cannot find block with consensus_timestamp: %s", tx.ConsensusTimestamp)
	}
	b := blocks.Blocks[0]
	bTimestamp, err := b.GetBlockTime()
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("invalid block time: %w", err)
	}

	currentBlock, err := c.FetchRawBlock(ctx, 0)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch currect block: %w", err)
	}
	confirmations := uint64(0)
	if currentBlock.Number > b.Number {
		confirmations = currentBlock.Number - b.Number
	}
	var txErr *string
	if tx.Result != RESULT_SUCCESS {
		txErr = &tx.Result
	}

	block := xclient.NewBlock(c.Asset.Chain, b.Number, b.Hash, bTimestamp)
	txInfo := xclient.NewTxInfo(block, c.Asset.GetChain(), tx.GetHash(), confirmations, txErr)
	txInfo.LookupId = string(tx.TransactionId)
	sourceAddress, err := tx.GetSourceAddress()
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get payment account: %w", err)
	}

	var allTransfers []resttypes.Transfer
	allTransfers = append(allTransfers, tx.Transfers...)
	allTransfers = append(allTransfers, tx.TokenTransfers...)

	contractToMovement := make(map[string]*xclient.Movement, 0)
	for i, transfer := range allTransfers {
		eventVariant := determineEventType(transfer, tx.Node)
		eventId := fmt.Sprintf("%d", i)
		if eventVariant == xclient.MovementVariantFee {
			continue
		}

		movement, ok := contractToMovement[transfer.TokenId]
		if !ok {
			newMovement := xclient.NewMovement(c.Asset.Chain, xc.ContractAddress(transfer.TokenId))
			contractToMovement[transfer.TokenId] = newMovement
			movement = newMovement
			txInfo.AddMovement(movement)
		}

		// determine direction
		if transfer.Amount < 0 {
			absAmount := uint64(transfer.Amount * -1)
			balanceChange := movement.AddSource(xc.Address(sourceAddress), xc.NewAmountBlockchainFromUint64(absAmount), nil)
			balanceChange.Event = xclient.NewEvent(eventId, eventVariant)
		} else {
			absAmount := uint64(transfer.Amount)
			balanceChange := movement.AddDestination(xc.Address(transfer.Account), xc.NewAmountBlockchainFromUint64(absAmount), nil)
			balanceChange.Event = xclient.NewEvent(eventId, eventVariant)
		}

		movement.Memo = tx.GetMemo()
	}

	txInfo.Fees = txInfo.CalculateFees()
	return *txInfo, nil
}

func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	acc, err := c.FetchAccountInfo(ctx, args.Address())
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch account info: %w", err)
	}

	contract, ok := args.Contract()
	if ok {
		for _, t := range acc.Balance.Tokens {
			if t.TokenId == string(contract) {
				return xc.NewAmountBlockchainFromUint64(t.Balance), nil
			}
		}
	} else {
		return xc.NewAmountBlockchainFromUint64(acc.Balance.Balance), nil
	}

	return xc.NewAmountBlockchainFromUint64(0), nil
}

func (c *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if contract != "" && contract != HBAR {
		url := c.IndexerUrl.JoinPath(ENDPOINT_TOKENS + "/" + string(contract))
		tokenInfo, err := Get[resttypes.TokenInfo](ctx, c, url)
		if err != nil {
			return 0, fmt.Errorf("failed to get token info: %w", err)
		}

		xcDecimals := xc.NewAmountBlockchainFromStr(tokenInfo.Decimals)
		return int(xcDecimals.Int().Int64()), nil
	} else {
		return int(c.Asset.GetChain().Decimals), nil
	}
}

func (c *Client) FetchRawBlocks(ctx context.Context, params url.Values) (*resttypes.BlocksInfo, error) {
	u := c.IndexerUrl.JoinPath(ENDPOINT_BLOCKS)
	u.RawQuery = params.Encode()
	blocks, err := Get[resttypes.BlocksInfo](ctx, c, u)
	return blocks, err
}

func (c *Client) FetchRawBlock(ctx context.Context, height uint64) (resttypes.Block, error) {
	u := c.IndexerUrl.JoinPath(ENDPOINT_BLOCKS)
	if height != 0 {
		u = u.JoinPath(fmt.Sprintf("%d", height))
		block, err := Get[resttypes.Block](ctx, c, u)
		return *block, err
	} else {
		params := url.Values{}
		params.Add(KEY_LIMIT, "1")
		blocks, err := c.FetchRawBlocks(ctx, params)
		if err != nil {
			return resttypes.Block{}, fmt.Errorf("failed to get blocks: %w", err)
		}

		if len(blocks.Blocks) == 0 {
			return resttypes.Block{}, fmt.Errorf("failed to fetch block: %d", height)
		}
		return blocks.Blocks[0], nil
	}
}

func (c *Client) FetchBlockTransactions(ctx context.Context, block resttypes.Block) ([]string, error) {
	u := c.IndexerUrl.JoinPath(ENDPOINT_TRANSACTIONS)
	params := url.Values{}
	ts, from := block.Timestamp.FromParam()
	params.Add(ts, from)
	ts, to := block.Timestamp.ToParam()
	params.Add(ts, to)

	hashes := make([]string, 0, block.Count)
	for range MAX_PAGES {
		if len(params) == 0 {
			break
		}

		u.RawQuery = params.Encode()
		transactions, err := Get[resttypes.TransactionsInfo](ctx, c, u)
		if err != nil {
			return []string{}, fmt.Errorf("failed to fetch transactions: %w", err)
		}

		for _, t := range transactions.Transactions {
			hashes = append(hashes, string(t.TransactionId))
		}

		// prepare parameters for the next page
		params = url.Values{}
		parts := strings.SplitN(transactions.Links.Next, "?", 2)
		if len(parts) == 2 {
			params, err = url.ParseQuery(parts[1])
			if err != nil {
				panic(err)
			}
		}
	}
	return hashes, nil
}

func (c *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, _ := args.Height()
	rawBlock, err := c.FetchRawBlock(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw block: %w", err)
	}
	transactions, err := c.FetchBlockTransactions(ctx, rawBlock)
	if err != nil {
		return nil, err
	}

	blockTime, err := rawBlock.GetBlockTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get block time: %w", err)
	}
	block := xclient.NewBlock(c.Asset.GetChain().Chain, rawBlock.Number, rawBlock.Hash, blockTime)
	return &xclient.BlockWithTransactions{
		Block:          *block,
		TransactionIds: transactions,
	}, nil
}

func (c *Client) RestLogger(ctx context.Context) *logrus.Entry {
	return c.Logger.Logger.WithContext(ctx).WithField("type", "rest")
}

func (c *Client) GrpcLogger(ctx context.Context) *logrus.Entry {
	return c.Logger.Logger.WithContext(ctx).WithField("type", "grpc")
}

func Get[R any](ctx context.Context, c *Client, url *url.URL) (*R, error) {
	logger := c.RestLogger(ctx)
	request, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	logger.WithField("request", request).Debug("sending request")

	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"response": response,
		"body":     string(body),
	}).Debug("got response")

	if response.StatusCode != http.StatusOK {
		var errRes resttypes.ErrorResponse
		if err := json.Unmarshal(body, &errRes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error body: %w", err)
		}
		return nil, fmt.Errorf("error response: %w", errRes)
	}

	var r R
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("failed to decode body: %w", err)
	}

	return &r, nil
}
