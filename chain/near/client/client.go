package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	types "github.com/cordialsys/crosschain/chain/near/client/types"
	nearerrors "github.com/cordialsys/crosschain/chain/near/errors"
	"github.com/cordialsys/crosschain/chain/near/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/sirupsen/logrus"
)

const (
	EventIdReceiptAction = "receipt"
	DefaultGasMultiplier = float64(2.0)
	DefaultRequestId     = "1"
	JsonRpcVersion       = "2.0"
	KeyId                = "id"
	KeyJsonrpc           = "jsonrpc"
	KeyMethod            = "method"
	KeyParams            = "params"
	KeyTypeEd25519       = "ed25519"
	KeyUrl               = "url"
	MethodBlock          = "block"
	MethodChunk          = "chunk"
	MethodGasPrice       = "gas_price"
	MethodProtocolConfig = "EXPERIMENTAL_protocol_config"
	MethodSendTx         = "send_tx"
	MethodTxStatus       = "EXPERIMENTAL_tx_status"
	MethodQuery          = "query"
	Near                 = "NEAR"
	SystemAccountId      = "system"
)

// Client for Template
type Client struct {
	Asset      xc.ITask
	Url        *url.URL
	IndexerUrl *url.URL
}

var _ xclient.Client = &Client{}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	chain := cfgI.GetChain()
	u, err := url.Parse(chain.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid rpc url: %w", err)
	}

	var indexerUrl *url.URL
	if chain.IndexerUrl == "" {
		indexerUrl = u
	} else {
		indexerUrl, err = u.Parse(chain.IndexerUrl)
		if err != nil {
			return nil, fmt.Errorf("invalid rpc url: %w", err)
		}
	}

	return &Client{
		Asset:      cfgI,
		Url:        u,
		IndexerUrl: indexerUrl,
	}, nil
}

type GasDetails struct {
	FeeEstimation xc.AmountBlockchain
	GasCost       xc.AmountBlockchain
}

func (client *Client) fetchTxCost(ctx context.Context, isToken bool) (GasDetails, error) {
	protocolConfigParams := types.ProtocolConfigParams{}
	protocolConfig, err := GetRpc[types.ProtocolConfig](ctx, client, MethodProtocolConfig, protocolConfigParams)
	if err != nil {
		return GasDetails{}, fmt.Errorf("failed to fetch protocol config: %w", err)
	}

	gasPriceParams := types.GasPriceParams{}
	gasPrice, err := GetRpc[types.GasPrice](ctx, client, MethodGasPrice, gasPriceParams)
	if err != nil {
		return GasDetails{}, fmt.Errorf("failed to fetch gas price: %w", err)
	}
	xcGasPrice := xc.NewAmountBlockchainFromStr(gasPrice.GasPrice)

	var gasCost uint64
	if isToken {
		gasCost = protocolConfig.RuntimeConfig.TransactionCosts.ActionCreationConfig.FunctionCallCost.Execution
	} else {
		gasCost = protocolConfig.RuntimeConfig.TransactionCosts.ActionCreationConfig.TransferCost.Execution
	}
	gasCost += protocolConfig.RuntimeConfig.TransactionCosts.ActionReceiptCreationConfig.Execution

	multiplier := client.Asset.GetChain().ChainGasMultiplier
	if multiplier < 0.01 {
		multiplier = DefaultGasMultiplier
	}
	xcGasCost := xc.NewAmountBlockchainFromUint64(gasCost)
	xcGasCost = xc.MultiplyByFloat(xcGasCost, multiplier)
	fee := xcGasCost.Mul(&xcGasPrice)

	return GasDetails{
		FeeEstimation: fee,
		GasCost:       xcGasCost,
	}, nil
}

// FetchTransferInput returns tx input for a Template tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	accessKeysParams := types.NewViewAccessKeyListParams(string(args.GetFrom()))
	accessKeys, err := GetRpc[types.AccessKeyList](ctx, client, MethodQuery, accessKeysParams)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch access keys list: %w", err)
	}

	publicKey, ok := args.GetPublicKey()
	if !ok {
		return nil, fmt.Errorf("near tx-input requires a valid public key")
	}

	b58Pk := base58.Encode(publicKey)
	expectedAccessKey := fmt.Sprintf("%s:%s", KeyTypeEd25519, b58Pk)
	var k *types.AccessKey
	for _, key := range accessKeys.Keys {
		if key.PublicKey == expectedAccessKey {
			k = &key.AccessKey
			break
		}
	}
	if k == nil {
		return nil, fmt.Errorf("failed to fetch nonce, no matching access key: %s", expectedAccessKey)
	}

	txInput := tx_input.NewTxInput()
	txInput.Nonce = k.Nonce + 1
	txInput.BlockHash = accessKeys.BlockHash

	contract, isToken := args.GetContract()
	if isToken {
		storageBalanceParams, err := types.NewStorageBalanceOfParams(
			string(contract),
			string(args.GetFrom()),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage_balance_of params: %w", err)
		}

		storageBalanceResult, err := GetRpc[types.StorageBalanceResult](ctx, client, MethodQuery, storageBalanceParams)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch storage balance: %w", err)
		}

		storageBalance, err := storageBalanceResult.GetStorageBalance()
		if err != nil {
			return nil, fmt.Errorf("failed to unrmarshal storage balance: %w", err)
		}

		if storageBalance.IsZero() {
			storageBoundsParams, err := types.NewStorageBalanceBoundsParams(string(contract), string(args.GetFrom()))
			if err != nil {
				return nil, fmt.Errorf("failed to create storage balance bounds params: %w", err)
			}
			storageBoundsResult, err := GetRpc[types.StorageBalanceBoundsResult](ctx, client, MethodQuery, storageBoundsParams)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch storage balance bounds: %w", err)
			}
			balanceBounds, err := storageBoundsResult.GetStorageBalanceBounds()
			if err != nil {
				return nil, fmt.Errorf("failed to unrmarshal storage balance bounds: %w", err)
			}

			minDepositAmount := xc.NewAmountBlockchainFromStr(balanceBounds.Min)
			txInput.RequiredDepopsit = minDepositAmount
		}
	}
	gasDetails, err := client.fetchTxCost(ctx, isToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tx cost: %w", err)
	}
	txInput.FeeEstimation = gasDetails.FeeEstimation
	txInput.GasCost = gasDetails.GasCost

	return txInput, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq) error {
	bz, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}
	bz64 := base64.StdEncoding.EncodeToString(bz)
	params := types.NewSendTxParams(bz64)

	// type aaa map[string]any
	_, err = GetRpc[any](ctx, client, MethodSendTx, params)
	if err != nil {
		return err
	}
	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, errors.New("not implemented")
}

func (client *Client) parseTokenTransferIntent(fc *types.FunctionCallAction, predecessorId string, contract string, eventId string) (txinfo.Movement, error, bool) {
	if !fc.IsTransfer() {
		return txinfo.Movement{}, nil, false
	}

	// don't include fee refund movement
	if predecessorId == SystemAccountId {
		return txinfo.Movement{}, nil, false
	}
	argsJson, err := base64.StdEncoding.DecodeString(fc.Args)
	if err != nil {
		return txinfo.Movement{}, fmt.Errorf("failed to decode funcion call args: %w", err), true
	}

	var args types.FunctionCallArgs
	if err := json.Unmarshal(argsJson, &args); err != nil {
		return txinfo.Movement{}, fmt.Errorf("failed to unmarshal function call args: %w", err), true
	}

	hrAmount, err := xc.NewAmountHumanReadableFromStr(args.Amount)
	if err != nil {
		return txinfo.Movement{}, fmt.Errorf("failed to parse funcion call amount: %w", err), true
	}
	xcAmount := hrAmount.ToBlockchain(0)
	movement := txinfo.NewMovement(client.Asset.GetChain().Chain, xc.ContractAddress(contract))
	movement.AddSource(xc.Address(predecessorId), xcAmount, nil)
	movement.AddDestination(xc.Address(args.ReceiverID), xcAmount, nil)

	eventVariant := txinfo.MovementVariantToken
	event := txinfo.NewEvent(eventId, eventVariant)
	movement.AddEventMeta(event)
	return *movement, nil, true
}

func (client *Client) processTransferAction(transfer types.TransferAction, from string, to string, eventId string) (txinfo.Movement, error, bool) {
	// don't include fee refund movement
	if from == SystemAccountId {
		return txinfo.Movement{}, nil, false
	}
	movement := txinfo.NewMovement(client.Asset.GetChain().Chain, "")
	hrAmount, err := xc.NewAmountHumanReadableFromStr(transfer.Deposit)
	if err != nil {
		return txinfo.Movement{}, fmt.Errorf("failed to decode transfer amount: %w", err), false
	}
	xcAmount := hrAmount.ToBlockchain(client.Asset.GetDecimals())
	movement.AddSource(xc.Address(from), xcAmount, nil)
	movement.AddDestination(xc.Address(to), xcAmount, nil)

	eventVariant := txinfo.MovementVariantNative
	event := txinfo.NewEvent(eventId, eventVariant)
	movement.AddEventMeta(event)
	return *movement, nil, true
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	senderId := ""
	hash := string(args.TxHash())
	parts := strings.Split(hash, "-")
	switch len(parts) {
	case 2:
		senderId = parts[0]
		hash = parts[1]
	case 1:
		sender, ok := args.Sender()
		if !ok {
			return txinfo.TxInfo{}, nearerrors.ErrMissingSenderId
		}
		senderId = string(sender)
	default:
		return txinfo.TxInfo{}, nearerrors.ErrMissingSenderId
	}

	txStatusParams := types.TxStatusParams{
		TxHash:          hash,
		SenderAccountId: senderId,
	}

	txStatus, err := GetIndexer[types.TxStatus](ctx, client, MethodTxStatus, txStatusParams)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get tx status: %w", err)
	}

	blockParams := types.NewBlockParamsById(txStatus.TransactionOutcome.BlockHash)
	block, err := GetIndexer[types.Block](ctx, client, MethodBlock, blockParams)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to fetch transaction block: %w", err)
	}

	header := block.Header
	blockTime := time.Unix(0, int64(header.Timestamp))
	xcBlock := txinfo.NewBlock(client.Asset.GetChain().Chain, header.Height, header.Hash, blockTime)

	latestBlockParams := types.NewLatestBlockParams()
	latestBlock, err := GetIndexer[types.Block](ctx, client, MethodBlock, latestBlockParams)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to fetch transaction block: %w", err)
	}

	txErr, err := txStatus.Status.GetError()
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get tx error: %w", err)
	}

	chain := client.Asset.GetChain()
	confirmations := latestBlock.Header.Height - block.Header.Height
	txInfo := txinfo.NewTxInfo(xcBlock, chain, string(args.TxHash()), confirmations, txErr)
	txInfo.Final = confirmations > uint64(client.Asset.GetChain().Confirmations.Final)

	fee := xc.NewAmountBlockchainFromUint64(0)
	receiptOutcomes := make(map[string]*types.ReceiptOutcome)
	for i := range txStatus.ReceiptsOutcome {
		receiptOutcomes[txStatus.ReceiptsOutcome[i].ID] = &txStatus.ReceiptsOutcome[i]
	}

	for receiptId, receipt := range txStatus.Receipts {
		if receipt.Receipt.Action == nil {
			continue
		}

		outcome, ok := receiptOutcomes[receipt.ReceiptID]
		if !ok {
			continue
		}
		burnt := xc.NewAmountBlockchainFromStr(outcome.Outcome.TokensBurnt)
		fee = fee.Add(&burnt)
		if outcome.Outcome.Status.Failure != nil {
			continue
		}

		for actionId, action := range receipt.Receipt.Action.Actions {
			from := receipt.PredecessorID
			to := receipt.ReceiverID
			eventId := fmt.Sprintf("%s-%d-%d", EventIdReceiptAction, receiptId, actionId)
			if action.Transfer != nil {
				movement, err, ok := client.processTransferAction(*action.Transfer, from, to, eventId)
				if err != nil {
					return txinfo.TxInfo{}, fmt.Errorf("failed to process receipt transfer action: %w", err)
				}
				if ok {
					txInfo.AddMovement(&movement)
				}

			} else if action.FunctionCall != nil {
				movement, err, ok := client.parseTokenTransferIntent(action.FunctionCall, from, to, eventId)
				if err != nil {
					return txinfo.TxInfo{}, fmt.Errorf("failed to parse token transfer: %w", err)
				}
				if ok {
					txInfo.AddMovement(&movement)
				}
			}
		}
	}

	// Make sure to include transaction outcome fee
	transactionTokensBurnt := xc.NewAmountBlockchainFromStr(txStatus.TransactionOutcome.Outcome.TokensBurnt)
	fee = fee.Add(&transactionTokensBurnt)
	txInfo.AddFee(xc.Address(txStatus.Transaction.SignerID), "", fee, nil)
	txInfo.Fees = txInfo.CalculateFees()
	txInfo.LookupId = fmt.Sprintf("%s-%s", txStatus.Transaction.SignerID, txInfo.Hash)
	return *txInfo, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	contract, ok := args.Contract()
	if !ok {
		params := types.NewViewAccountParams(string(args.Address()))
		account, err := GetRpc[types.Account](ctx, client, MethodQuery, params)
		return xc.NewAmountBlockchainFromStr(account.Amount), err
	} else {
		params, err := types.NewFtBalanceOfParams(string(args.Address()), string(contract))
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to create token balance params: %w", err)
		}
		balanceOfResponse, err := GetRpc[types.FtBalanceOf](ctx, client, MethodQuery, params)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("failed to fetch token balance: %w", err)
		}

		balance := "0"
		err = json.Unmarshal(balanceOfResponse.Result, &balance)
		return xc.NewAmountBlockchainFromStr(balance), err
	}
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if contract == "" || contract == Near {
		return int(client.Asset.GetChain().Decimals), nil
	}

	params, err := types.NewTokenMetadataParams(string(contract))
	if err != nil {
		return 0, err
	}

	result, err := GetRpc[types.TokenMetadataResult](ctx, client, MethodQuery, params)
	if err != nil {
		return 0, err
	}

	metadata, err := result.GetTokenMetadata()
	if err != nil {
		return 0, err
	}
	return metadata.Decimals, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	height, ok := args.Height()
	var blockParams types.BlockParams
	if !ok {
		blockParams = types.NewLatestBlockParams()
	} else {
		blockParams = types.NewBlockParamsById(fmt.Sprintf("%d", height))
	}
	block, err := GetIndexer[types.Block](ctx, client, MethodBlock, blockParams)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block: %w", err)
	}

	transactions := make([]string, 0)
	for _, chunk := range block.Chunks {
		chunkParams := types.ChunkParams{
			ChunkId: chunk.ChunkHash,
		}

		chunkStatus, err := GetIndexer[types.ChunkStatus](ctx, client, MethodChunk, chunkParams)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chunk status: %w", err)
		}
		for _, tx := range chunkStatus.Transactions {
			transactions = append(transactions, tx.Hash)
		}
	}

	blockTime := time.Unix(0, int64(block.Header.Timestamp))
	xcBlock := txinfo.NewBlock(client.Asset.GetChain().Chain, block.Header.Height, block.Header.Hash, blockTime)
	return &txinfo.BlockWithTransactions{
		Block:          *xcBlock,
		TransactionIds: transactions,
		SubBlocks:      []*txinfo.SubBlockWithTransactions{},
	}, nil
}

func GetRpc[T any](ctx context.Context, client *Client, method string, params types.Params) (*T, error) {
	return get[T](ctx, client.Url.String(), method, params)
}

func GetIndexer[T any](ctx context.Context, client *Client, method string, params types.Params) (*T, error) {
	return get[T](ctx, client.IndexerUrl.String(), method, params)
}

func get[T any](ctx context.Context, url string, method string, params types.Params) (*T, error) {
	requestParams, err := params.ToParams()
	if err != nil {
		return nil, fmt.Errorf("failed to get convert params: %w", err)

	}

	logger := logrus.WithFields(logrus.Fields{
		KeyMethod: method,
		KeyParams: requestParams,
		KeyUrl:    url,
	})

	paramsWrapper := map[string]any{
		KeyMethod:  method,
		KeyJsonrpc: JsonRpcVersion,
		KeyId:      DefaultRequestId,
		KeyParams:  requestParams,
	}
	paramsBz, err := json.Marshal(paramsWrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	logger.WithField("params", paramsWrapper).Trace("sending request")
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(paramsBz))
	if err != nil {
		return nil, fmt.Errorf("failed to create a request: %w", err)
	}
	request.Header.Add("content-type", "application/json")
	request.WithContext(ctx)

	rawResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed post: %w", err)
	}
	defer rawResponse.Body.Close()
	logger.WithField("response", rawResponse).Trace("got response")

	body, err := io.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if rawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to post request, status: %d, body: %s", rawResponse.StatusCode, string(body))
	}
	logger.WithField("body", string(body)).Trace("body")

	var response types.Response[T]
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if response.Err != nil {
		return nil, fmt.Errorf("rpc error: %w", response.Err)
	}

	return &response.Result, nil
}
