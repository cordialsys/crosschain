package tron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	xclient "github.com/cordialsys/crosschain/client"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
)

var _ xclient.Client = &Client{}

const (
	TRANSFER_EVENT_HASH_HEX    = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	CREATE_TRANSACTION         = "createtransaction"
	STAKE_TRANSACTION_MAX_WAIT = time.Second * 30
	KEY_FEE_PER_BANDWIDTH      = "getTransactionFee"
)

// Client for Template
type Client struct {
	// client *client.GrpcClient
	client *httpclient.Client

	chain *xc.ChainConfig
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

func (client *Client) FetchBaseInput(ctx context.Context, params httpclient.CreateInputParams) (*txinput.TxInput, error) {
	input := new(txinput.TxInput)

	// Getting blockhash details from the CreateTransfer endpoint as TRON uses an unusual hashing algorithm (SHA2256SM3), so we can't do a minimal
	// retrieval and just get the blockheaders
	dummyTx, err := client.client.CreateTransaction(params)
	if err != nil {
		return nil, err
	}

	input.RefBlockBytes = dummyTx.RawData.RefBlockBytes
	input.RefBlockHash = dummyTx.RawData.RefBlockHashBytes
	// set timeout period
	input.Timestamp = time.Now().Unix()
	input.Expiration = time.Now().Add(txinput.TX_TIMEOUT).Unix()

	maxFee := client.chain.GasBudgetDefault.ToBlockchain(client.chain.Decimals)
	input.MaxFee = maxFee

	return input, nil
}

func (client *Client) EstimateTransactionFee(ctx context.Context, transaction xc.Tx, sender xc.Address) (uint64, error) {
	bz, err := transaction.Serialize()
	if err != nil {
		return 0, fmt.Errorf("failed to serialize transfer for fee estimation: %w", err)
	}
	txSize := len(bz)
	accountResources, err := client.client.GetAccountResources(string(sender))
	if err != nil {
		return 0, fmt.Errorf("failed to get account resources: %w", err)
	}
	chainParameters, err := client.client.GetChainParameters()
	if err != nil {
		return 0, fmt.Errorf("failed to get chain parameters: %w", err)
	}

	availableBandwidth := accountResources.GetAvailableBandwith()
	bandwidthRequired := txSize - int(availableBandwidth)
	// free transfer
	if bandwidthRequired <= 0 {
		return 0, nil
	}

	feePerBandwidth, ok := chainParameters.GetParam(KEY_FEE_PER_BANDWIDTH)
	if !ok {
		return 0, errors.New("failed to get bandwidth price")
	}

	return uint64(bandwidthRequired * feePerBandwidth), nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	params := httpclient.CreateTransactionParams{
		From:   args.GetFrom(),
		To:     args.GetTo(),
		Amount: args.GetAmount(),
	}

	baseInput, err := client.FetchBaseInput(ctx, params)
	if err != nil {
		return nil, err
	}

	builder, err := NewTxBuilder(client.chain.Base())
	if err != nil {
		return nil, fmt.Errorf("failed to create a new tx builder: %w", err)
	}
	dummyTx, err := builder.Transfer(args, baseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create dummy transfer tx for fee estimation: %w", err)
	}

	fee, err := client.EstimateTransactionFee(ctx, dummyTx, args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to estimate transaction fee: %w", err)
	}
	baseInput.MaxFee = xc.NewAmountBlockchainFromUint64(fee)

	return baseInput, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.chain.Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Tron tx
// Submission of unstake/stake/withdraw transactions relies on proper handling
// of "FailedPrecondition" error. It's modeled this way to avoid blocking and match
// treasury behavior.
func (client *Client) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq) error {
	// TODO: Refactor SubmitTx interface to accept SubmitTxReq instead of tx
	// This check will always return 'ok == true' at the moment
	metaBz, ok, err := tx.GetMetadata()
	if err != nil {
		return fmt.Errorf("failed to get tx metadata: %w", err)
	}

	// no metadata provided, submit standard tx
	if !ok {
		bz, err := tx.Serialize()
		if err != nil {
			return err
		}
		_, err = client.client.BroadcastHex(hex.EncodeToString(bz))
		return err
	}

	var txMeta BroadcastMetadata
	if err = json.Unmarshal(metaBz, &txMeta); err != nil {
		return fmt.Errorf("failed to unmarshal tx metadata: %w", err)
	}

	// traditional, single tx submit
	if len(txMeta.TransactionsData) == 1 {
		bz, err := tx.Serialize()
		if err != nil {
			return err
		}
		_, err = client.client.BroadcastHex(hex.EncodeToString(bz))
		return err
	} else if len(txMeta.TransactionsData) == 2 {
		// staking/unstaking/withdrawal could contain two transactions to submit
		txbytes, err := tx.Serialize()
		if err != nil {
			return fmt.Errorf("failed to deserialize tx: %w", err)
		}

		meta := txMeta.TransactionsData[0]
		hash := meta.Hash
		infoArgs := txinfo.NewArgs(xc.TxHash(hash))
		info, err := client.FetchTxInfo(context.Background(), infoArgs)
		// first transaction is not on chain yet, we have to submit it
		if err != nil && strings.Contains(err.Error(), string(xcerrors.TransactionNotFound)) {
			// submit first tx and return
			bz := txbytes[:meta.Length]
			_, err = client.client.BroadcastHex(hex.EncodeToString(bz))
			if err != nil {
				return fmt.Errorf("failed first tx submission: %w", err)
			}

			return xcerrors.FailedPreconditionf("required resubmission")
		}

		// FetchTxInfo returned error
		if err != nil {
			return fmt.Errorf("failed to fetch tx info: %w", err)
		}

		// We found info about transaction
		isValidTx := (info.Error == nil || *info.Error == "")
		if isValidTx {
			// submit second tx and return
			bz := txbytes[meta.Length:]
			_, err = client.client.BroadcastHex(hex.EncodeToString(bz))
			return err
		} else {
			return fmt.Errorf("on chain transaction error: %s", *info.Error)
		}
	}

	return errors.New("tron submittx supports max 2 transactions")
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

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	rpcClient, err := client.ConnectEvmJsonRpc(ctx)
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}
	tx, err := client.client.GetTransactionByID(string(txHash))
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}

	info, err := client.client.GetTransactionInfoByID(string(txHash))
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}
	evmTx, err := rpcClient.TransactionReceipt(ctx, common.HexToHash(string(txHash)))
	if err != nil {
		return txinfo.LegacyTxInfo{}, fmt.Errorf("failed to get transaction by hash from EVM RPC: %v", err)
	}

	latestBlockResp, err := client.client.GetBlockByLatest(1)
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
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
			source := new(txinfo.LegacyTxInfoEndpoint)
			source.Address = from
			source.Amount = amount
			source.Asset = string(client.chain.Chain)
			source.NativeAsset = client.chain.Chain

			destination := new(txinfo.LegacyTxInfoEndpoint)
			destination.Address = to
			destination.Amount = amount
			destination.Asset = string(client.chain.Chain)
			destination.NativeAsset = client.chain.Chain
			destination.Event = txinfo.NewEvent("", txinfo.MovementVariantNative)

			sources = append(sources, source)
			destinations = append(destinations, destination)
		}
	}

	blockHash := evmTx.BlockHash.String()
	// Tron natively doesn't use 0x prefix.
	blockHash = strings.TrimPrefix(blockHash, "0x")

	txInfo := txinfo.LegacyTxInfo{
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

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, args.TxHash())
	if err != nil {
		return txinfo.TxInfo{}, err
	}

	// remap to new tx
	return txinfo.TxInfoFromLegacy(client.chain, legacyTx, txinfo.Account), nil
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
func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
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

	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
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
