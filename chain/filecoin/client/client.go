package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/filecoin/client/types"
	filtx "github.com/cordialsys/crosschain/chain/filecoin/tx"
	"github.com/cordialsys/crosschain/chain/filecoin/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	log "github.com/sirupsen/logrus"
	"github.com/stellar/go/support/time"
)

// Client for Filecoin
type Client struct {
	Url          string
	HttpClient   *http.Client
	Asset        xc.ITask
	Logger       *log.Entry
	MaxGasFeeCap xc.AmountBlockchain
	MaxGasLimit  uint64
}

const DefaultGasLimit = 15_000_000

var _ xclient.FullClient = &Client{}
var DefaultMaxGasPrice = xc.NewAmountBlockchainFromUint64(10_000_000)

// NewClient returns a new Filecoin Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	logger := log.WithFields(log.Fields{
		"chain":   cfg.Chain,
		"rpc":     cfg.URL,
		"network": cfg.Net,
	})

	maxGasPrice := cfg.ChainMaxGasPrice
	hrMaxGasPrice := xc.NewAmountHumanReadableFromFloat(maxGasPrice)
	xcMaxGasPrice := hrMaxGasPrice.ToBlockchain(cfg.Decimals)
	if xcMaxGasPrice.IsZero() {
		xcMaxGasPrice = DefaultMaxGasPrice
	}
	logger.Infof("using MaxGasFeeCap: %s", xcMaxGasPrice.String())

	gasLimit := cfg.ChainGasTip
	if gasLimit == 0 {
		gasLimit = DefaultGasLimit
	}
	logger.Infof("using MaxGasLimit: %v", gasLimit)

	return &Client{
		Url:          cfg.URL,
		HttpClient:   http.DefaultClient,
		Asset:        cfgI,
		Logger:       logger,
		MaxGasFeeCap: xcMaxGasPrice,
		MaxGasLimit:  gasLimit,
	}, nil
}

// Filecoin transaction requires gas estimation to be done before submitting the transaction.
// Because of this we have to execute multiple API calls to get the gas estimation:
// 1. `ChainHead`
// 2. `MpoolGetNonce`
// 3. `GasEstimateMessageGas`
//
// Filecoin fees consist of two parts:
// - BurnFee: GasUsed * BaseFee
// - MinerFee: GasLimit * GasPremium, where GasPremium is capped by MaxGasFeeCap
// Because of this, we can limit max fee spent by setting cap on `GasLimit` and `MaxGasFeeCap`
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	chainHeadParams := types.NewEmptyParams(types.MethodChainHead)
	chainHeadResponse := types.NewChainHeadResponse()
	err := Post(client, chainHeadParams, chainHeadResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain head: %w", err)
	}
	tipset := chainHeadResponse.Result.TipsetKey

	from := string(args.GetFrom())
	// `to` is required for `Filecoin.EstimateGasFees` method
	to := string(args.GetTo())
	if len(to) == 0 {
		to = from
	}
	mpoolGetNonceParams := types.NewParams(types.MethodMpoolGetNonce, types.MpoolGetNonce{
		Address: from,
	})
	mpoolGetNonceResponse := types.NewMpoolGetNonceResponse()
	err = Post(client, mpoolGetNonceParams, mpoolGetNonceResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}
	nonce := uint64(*mpoolGetNonceResponse.Result)

	message := types.Message{
		Version:    42,
		To:         to,
		From:       from,
		Value:      args.GetAmount().String(),
		Nonce:      nonce,
		GasFeeCap:  "0",
		GasPremium: "0",
	}

	// Create a new gas estimation request for a message using the Filecoin API.
	// The response will contain the estimated gas fees for the message,
	// GasFeeCap is the maximum fee that the sender is willing to pay for gas,
	// and GasPremium is the fee that the sender is willing to pay to miners.
	//
	// Max GasFeeCap can be limited by the MaxFee field.
	maxFee := client.Asset.GetChain().ChainMaxGasPrice
	hrMaxFee := xc.NewAmountHumanReadableFromFloat(maxFee)
	gasEstimateMessageGas := types.NewParams(types.MethodGasEstimateMessageGas, types.GasEstimateMessageGas{
		Message:   message,
		MaxFee:    types.MaxFee{MaxFee: hrMaxFee.String()},
		TipsetKey: tipset,
	})
	gasEstimateMessageGasResponse := types.NewGasEstimateMessageGasResponse()
	err = Post(client, gasEstimateMessageGas, gasEstimateMessageGasResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate gas: %w", err)
	}
	msgWithFees := gasEstimateMessageGasResponse.Result

	gasFeeCap := xc.NewAmountBlockchainFromStr(msgWithFees.GasFeeCap)
	if gasFeeCap.Cmp(&client.MaxGasFeeCap) == 1 {
		gasFeeCap = client.MaxGasFeeCap
	}
	gasPremium := xc.NewAmountBlockchainFromStr(msgWithFees.GasPremium)
	if msgWithFees.GasLimit > client.MaxGasLimit {
		msgWithFees.GasLimit = client.MaxGasLimit
	}
	return &tx_input.TxInput{
		Nonce:      nonce,
		GasLimit:   msgWithFees.GasLimit,
		GasFeeCap:  gasFeeCap,
		GasPremium: gasPremium,
	}, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a filecoin tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	filTx := tx.(*filtx.Tx)
	var msg types.Message
	msg.FromTxMsg(&filTx.Message)
	mpoolPushParams := types.NewParams(types.MethodMpoolPush, types.MpoolPush{
		Message: msg,
		Signature: types.Signature{
			Type: filTx.Signature.Type,
			Data: filTx.Signature.Data,
		},
	})
	mpoolPushResponse := types.NewMpoolPushResponse()
	err := Post(client, mpoolPushParams, mpoolPushResponse)
	if err != nil {
		return fmt.Errorf("failed submit tx: %w", err)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	return xc.LegacyTxInfo{}, errors.New("not implemented")
}

// Fetch Filecoin transaction info
// Because of how Filecoin works, we need to fetch multiple data points to get the transaction info:
// 1. `ChainGetMessage` - get gas info
// 2. `StateSearchMsg` - get used gas
// 3. `ChainGetBlock` - get block data
// 4. `ChainHead` - calculate confirmations
//
// Filecoin Fees are calculated as follows:
// 1. GasLimit * GasPremium - miner fee
// 2. GasUsed * BaseFee - burn fee
// 3. MinerFee + BurnFee - total fee
//
// There is a small penalty for overestimating the gas limit, which is not included in calculations
// at the moment
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	messageCid := types.NewCid(string(txHash))
	chainGetMessageParams := types.NewParams(types.MethodChainGetMessage, types.ChainGetMessage{
		Cid: messageCid,
	})
	chainGetMessageResponse := types.NewChainGetMessageResponse()
	err := Post(client, chainGetMessageParams, chainGetMessageResponse)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get chain message: %w", err)
	}
	msg := chainGetMessageResponse.Result

	stateSearchMsgParams := types.NewParams(types.MethodStateSearchMsg, types.StateSearchMsg{
		TipSetKey:     []types.Cid{},
		Message:       messageCid,
		Limit:         -1,
		AllowReplaced: true,
	})
	stateSearchMsgResponse := types.NewStateSearchMsgResponse()
	err = Post(client, stateSearchMsgParams, stateSearchMsgResponse)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to search message state: %w", err)
	}
	msgState := stateSearchMsgResponse.Result
	if msgState == nil {
		return xclient.TxInfo{}, errors.New("transaction not found")
	}

	// Filecoin head is a set of bloks called tipset.
	// Populate the block data with first block of the set
	chainGetBlockParams := types.NewParams(types.MethodChainGetBlock, types.ChainGetBlock{
		Cid: msgState.TipSet[0],
	})
	chainGetBlockResponse := types.NewChainGetBlockResponse()
	err = Post(client, chainGetBlockParams, chainGetBlockResponse)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get chain block: %w", err)
	}

	chainHeadParams := types.NewEmptyParams(types.MethodChainHead)
	chainHeadResponse := types.NewChainHeadResponse()
	err = Post(client, chainHeadParams, chainHeadResponse)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to get chain head: %w", err)
	}
	chainHeadHeight := chainHeadResponse.Result.Height

	chain := client.Asset.GetChain().Chain
	sHash := string(txHash)
	blockData := chainGetBlockResponse.Result
	block := xclient.NewBlock(chain, blockData.Height, sHash, time.MillisFromInt64(blockData.Timestamp).ToTime())
	txInfo := xclient.TxInfo{
		Name:          xclient.NewTransactionName(chain, sHash),
		XChain:        chain,
		Hash:          sHash,
		Block:         block,
		Confirmations: chainHeadHeight - block.Height,
	}

	if msgState.Receipt.ExitCode != 0 {
		errorMsg := fmt.Sprintf("error code %v", msgState.Receipt.ExitCode)
		txInfo.Error = &errorMsg
		txInfo.State = xclient.Failed
	}

	sourceAddress := xc.Address(msg.From)
	movement := xclient.NewMovement(chain, "")
	amount := xc.NewAmountBlockchainFromStr(msg.Value)
	movement.AddSource(xc.Address(msg.From), amount, nil)
	movement.AddDestination(sourceAddress, amount, nil)
	txInfo.AddMovement(movement)

	gasLimit, ok := xc.NewAmountBlockchainFromInt64(int64(msg.GasLimit))
	if !ok {
		return xclient.TxInfo{}, fmt.Errorf("failed to convert gas limit: %w", err)
	}
	gasPremium := xc.NewAmountBlockchainFromStr(msg.GasPremium)
	minerFee := gasLimit.Mul(&gasPremium)

	gasUsed := xc.NewAmountBlockchainFromUint64(msgState.Receipt.GasUsed)
	baseFee := xc.NewAmountBlockchainFromStr(blockData.ParentBaseFee)
	burnFee := gasUsed.Mul(&baseFee)

	feeAmount := minerFee.Add(&burnFee)
	txInfo.AddFee(sourceAddress, "", feeAmount, nil)
	txInfo.Fees = txInfo.CalculateFees()
	txInfo.Final = int(txInfo.Confirmations) > client.Asset.GetChain().ConfirmationsFinal

	return txInfo, nil
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalance(ctx, address)
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	params := types.NewParams(types.MethodWalletBallance, types.WalletBalance{
		Address: string(address),
	})

	response := types.NewResponse[string]()
	err := Post(client, params, response)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch balance: %w", err)
	}

	amount := xc.NewAmountBlockchainFromStr(*response.Result)
	return amount, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return int(client.Asset.GetChain().GetDecimals()), nil
}

// Filecoin relies on the `TipSet` as a chain head. Once tipset can include multiple blocks.
// Instead of fetching block transactions, we will return all transactions included in the TipSet.
//
// This method uses few json-rpc calls to get the data:
// 1. Fetch bock hashes at the specified height + 1, method: `Filecoin.ChainGetTipSetAfterHeight`
// 2. Fetch parent tipset messages, method: `Filecoin.ChainGetParentMessages`
// 3. Fetch block data, method: `Filecoin.ChainGetBlock`. Required to get the block timestamp.
//
// It is really important to use `Filecoin.ChainGetTipSetAfterHeight` method instead of
// `Filecoin.ChainGetTipSetMessages` because the later one can include duplicated messages.
// TODO: Once `ChainGetMessagesInTipset` method is fixed, we should use it instead of `ChainGetParentMessages`
func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, ok := args.Height()
	var blockCid types.Cid
	if ok {
		chainGetTipSetAfterHeightParams := types.NewParams(types.MethodChainGetTipSetAfterHeight, types.ChainGetTipSetAfterHeight{
			// FIXIT: We are using height + 1 because of the use of `ChainGetParentMessages` method.
			// We could use `ChainGetMessagesInTipset` but most rpcs return `MethodNotFound` error.
			Height: height + 1,
		})
		chainGetTipSetAfterHeightResponse := types.NewChainGetTipSetAfterHeightResponse()
		err := Post(client, chainGetTipSetAfterHeightParams, chainGetTipSetAfterHeightResponse)
		if err != nil {
			return &xclient.BlockWithTransactions{}, fmt.Errorf("failed to get tipset after height: %w", err)
		}
		tipset := chainGetTipSetAfterHeightResponse.Result.TipsetKey
		if len(tipset) == 0 {
			return &xclient.BlockWithTransactions{}, errors.New("tipset contains no blocks")
		}
		blockCid = chainGetTipSetAfterHeightResponse.Result.TipsetKey[0]
	} else {
		chainGetHeadParams := types.NewEmptyParams(types.MethodChainHead)
		chainGetHeadResponse := types.NewChainHeadResponse()
		err := Post(client, chainGetHeadParams, chainGetHeadResponse)
		if err != nil {
			return &xclient.BlockWithTransactions{}, fmt.Errorf("failed to get chain head: %w", err)
		}
		tipset := chainGetHeadResponse.Result.TipsetKey
		if len(tipset) == 0 {
			return &xclient.BlockWithTransactions{}, errors.New("tipset contains no blocks")
		}
		blockCid = chainGetHeadResponse.Result.TipsetKey[0]
	}

	chainGetParentMessagesParams := types.NewParams(types.MethodChainGetParentMessages, types.ChainGetParentMessages{
		Cid: blockCid,
	})
	chainGetParentMessagesResponse := types.NewChainGetParentMessagesResponse()
	err := Post(client, chainGetParentMessagesParams, chainGetParentMessagesResponse)
	if err != nil {
		return &xclient.BlockWithTransactions{}, fmt.Errorf("failed to get tipset messages: %w", err)
	}
	if chainGetParentMessagesResponse.Result == nil {
		return &xclient.BlockWithTransactions{}, errors.New("failed to get tipset messages")
	}

	// Filecoin head is a set of bloks called tipset.
	// Populate the block data with first block of the set
	chainGetBlockParams := types.NewParams(types.MethodChainGetBlock, types.ChainGetBlock{
		Cid: blockCid,
	})
	chainGetBlockResponse := types.NewChainGetBlockResponse()
	err = Post(client, chainGetBlockParams, chainGetBlockResponse)
	if err != nil {
		return &xclient.BlockWithTransactions{}, fmt.Errorf("failed to get block: %w", err)
	}
	blockTimestamp := time.MillisFromInt64(chainGetBlockResponse.Result.Timestamp)

	parentHash := chainGetBlockResponse.Result.Parents[0]
	block := xclient.NewBlock(client.Asset.GetChain().Chain, height, parentHash.Value, blockTimestamp.ToTime())
	transactions := make([]string, 0, len(*chainGetParentMessagesResponse.Result))
	for _, message := range *chainGetParentMessagesResponse.Result {
		transactions = append(transactions, message.Cid.Value)
	}
	return &xclient.BlockWithTransactions{
		Block:          *block,
		TransactionIds: transactions,
	}, nil
}

// Create and send a Json-RPC request to the Filecoin API
func Post[P any, R any](client *Client, params types.Params[P], response *types.Response[R]) error {
	payload, err := json.Marshal(&params)
	logger := client.Logger.WithField("method", params.Method)
	if err != nil {
		return fmt.Errorf("failed to marshall request payload: %w", err)
	}

	request, err := http.NewRequest("POST", client.Url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	logger.WithField("payload", string(payload)).Debug("sending request")
	resp, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send request, status: %s", resp.Status)
	}

	buff, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	logger.WithField("response", string(buff)).Debug("got response")
	err = json.Unmarshal(buff, response)
	if err != nil {
		return fmt.Errorf("failed to decode response body: %w, buff: %s", err, string(buff))
	}

	if response.IsError() {
		return &response.Error
	}

	return nil
}
