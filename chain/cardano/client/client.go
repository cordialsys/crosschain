package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	log "github.com/sirupsen/logrus"
)

const (
	POST = "POST"
	GET  = "GET"
	// Cardano uses lovelace as the smallest unit of ada
	// 1 lovelace = 0.000001 ada
	LOVELACE    = "lovelace"
	API_VERSION = "/api/v0"
)

// Client for Template
type Client struct {
	Asset      xc.ITask
	Url        string
	Network    string
	Logger     *log.Entry
	ProjectId  string
	HttpClient *http.Client
}

var _ xclient.Client = &Client{}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	url := cfg.GetChain().URL
	if url == "" {
		return nil, errors.New("rpc url is empty")
	}

	network := cfg.GetChain().Network
	if network == "" {
		return nil, errors.New("rpc url is empty")
	}

	logger := log.WithFields(log.Fields{
		"chain":   cfg.Chain,
		"rpc":     url,
		"network": network,
	})

	return &Client{
		Asset:   cfgI,
		Url:     url,
		Network: network,
		Logger:  logger,
		// Not all providers require api key
		ProjectId:  cfg.Auth2.LoadOrBlank(),
		HttpClient: http.DefaultClient,
	}, nil
}

func (client *Client) FetchProtocolParameters(ctx context.Context) (types.ProtocolParameters, error) {
	var protocolParameters types.ProtocolParameters
	err := client.Request(ctx, GET, "/epochs/latest/parameters", nil, &protocolParameters)

	return protocolParameters, err
}

func (client *Client) FetchUtxos(ctx context.Context, address xc.Address, contract xc.ContractAddress) ([]types.Utxo, error) {
	path := fmt.Sprintf("/addresses/%s/utxos/%s?order=desc", string(address), string(contract))
	var response []types.Utxo
	err := client.Request(ctx, GET, path, nil, &response)
	return response, err
}

// 1. Sort utxos by amount descending
// 2. Get minimum utxo set that adds up to the target amount
func GetMinUtxoSet(utxos []types.Utxo, targetAmount xc.AmountBlockchain, contract xc.ContractAddress) []types.Utxo {
	slices.SortFunc(utxos, func(lhs types.Utxo, rhs types.Utxo) int {
		amountL := lhs.GetAssetAmount(contract)
		amountR := rhs.GetAssetAmount(contract)

		return amountR.Cmp(&amountL)
	})

	utxoSet := make([]types.Utxo, 0)
	balance := xc.NewAmountBlockchainFromUint64(0)
	for _, utxo := range utxos {
		if balance.Cmp(&targetAmount) >= 0 {
			break
		}

		amount := utxo.GetAssetAmount(contract)
		balance = balance.Add(&amount)
		utxoSet = append(utxoSet, utxo)
	}
	return utxoSet
}

// FetchTransferInput returns tx input for a Cardano transfer
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	decimals, err := client.FetchDecimals(ctx, LOVELACE)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch decimals: %w", err)
	}

	contract, ok := args.GetContract()
	if !ok {
		contract = LOVELACE
	}

	utxos, err := client.FetchUtxos(ctx, args.GetFrom(), contract)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch utxos: %w", err)
	}

	// Include the fee in target amount
	cfg := client.Asset.GetChain()
	feeLimit := cfg.FeeLimit
	xfeeLimit := feeLimit.ToBlockchain(int32(decimals))
	targetAmount := args.GetAmount()
	targetAmount.Add(&xfeeLimit)

	utxos = GetMinUtxoSet(utxos, targetAmount, contract)

	var latestBlock types.Block
	err = client.Request(ctx, GET, "/blocks/latest", nil, &latestBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block info: %w", err)
	}

	protocolParams, err := client.FetchProtocolParameters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch protocol parameters: %w", err)
	}

	baseFee := protocolParams.FixedFee
	feePerByte := protocolParams.FeePerByte

	return &tx_input.TxInput{
		Utxos:            utxos,
		FixedFee:         xc.NewAmountBlockchainFromUint64(baseFee),
		FeePerByte:       xc.NewAmountBlockchainFromUint64(feePerByte),
		Slot:             latestBlock.Slot,
		MinUtxo:          xc.NewAmountBlockchainFromStr(protocolParams.MinUtxoValue),
		CoinsPerUtxoWord: xc.NewAmountBlockchainFromStr(protocolParams.CoinsPerUtxoWord),
	}, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	bytes, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}

	var response string
	err = client.Request(ctx, POST, "/tx/submit", bytes, &response)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, errors.New("deprecated")
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	var latestBlock types.Block
	err := client.Request(ctx, GET, "/blocks/latest", nil, &latestBlock)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch latest block: %w", err)
	}

	var transactionInfo types.TransactionInfo
	transactionPath := fmt.Sprintf("/txs/%s", string(txHash))
	err = client.Request(ctx, GET, transactionPath, nil, &transactionInfo)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch transaction info: %w", err)
	}

	var blockInfo types.Block
	blockPath := fmt.Sprintf("/blocks/%d", transactionInfo.BlockHeight)
	err = client.Request(ctx, GET, blockPath, nil, &blockInfo)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch block info: %w", err)
	}

	chain := client.Asset.GetChain().Chain
	timestamp := time.Unix(blockInfo.Time, 0)
	block := xclient.NewBlock(
		chain,
		uint64(transactionInfo.BlockHeight),
		blockInfo.Hash,
		timestamp,
	)

	txInfo := xclient.NewTxInfo(
		block,
		client.Asset.GetChain(),
		string(txHash),
		blockInfo.Confirmations,
		nil,
	)

	var transactionUtxos types.TransactionUtxos
	transactionUtxosPath := fmt.Sprintf("%s/utxos", transactionPath)
	err = client.Request(ctx, GET, transactionUtxosPath, nil, &transactionUtxos)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch transaction utxos: %w", err)
	}
	contractToMovement := make(map[xc.ContractAddress]*xclient.Movement)
	for _, input := range transactionUtxos.Inputs {
		addr := xc.Address(input.Address)
		for _, amount := range input.Amounts {
			contract := xc.ContractAddress(amount.Unit)
			if contractToMovement[contract] == nil {
				contractToMovement[contract] = xclient.NewMovement(
					xc.ADA,
					contract,
				)
			}
			contractToMovement[contract].AddSource(addr, xc.NewAmountBlockchainFromStr(amount.Quantity), nil)
		}
	}

	for _, output := range transactionUtxos.Outputs {
		addr := xc.Address(output.Address)
		for _, amount := range output.Amounts {
			contract := xc.ContractAddress(amount.Unit)
			if contractToMovement[contract] == nil {
				contractToMovement[contract] = xclient.NewMovement(
					xc.ADA,
					contract,
				)
			}
			contractToMovement[contract].AddDestination(addr, xc.NewAmountBlockchainFromStr(amount.Quantity), nil)
		}
	}

	movements := slices.Collect(maps.Values(contractToMovement))
	txInfo.Movements = movements

	feeAmount := xc.NewAmountBlockchainFromStr(transactionInfo.Fees)
	feeAccount := xc.Address(transactionUtxos.Inputs[0].Address)
	txInfo.AddFee(feeAccount, LOVELACE, feeAmount, nil)

	return *txInfo, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	path := fmt.Sprintf("/addresses/%s", string(args.Address()))
	var getAddressInfoResponse types.GetAddressInfoResponse
	err := client.Request(ctx, GET, path, nil, &getAddressInfoResponse)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch address info: %w", err)
	}

	contract, ok := args.Contract()
	if !ok {
		contract = LOVELACE
	}

	for _, amount := range getAddressInfoResponse.Amounts {
		if amount.Unit == string(contract) {
			return xc.NewAmountBlockchainFromStr(amount.Quantity), nil
		}
	}

	return xc.NewAmountBlockchainFromUint64(0), nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 6, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, ok := args.Height()
	var blockPath string
	if ok {
		blockPath = fmt.Sprintf("/blocks/%d", height)
	} else {
		blockPath = "/blocks/latest"
	}
	var block types.Block
	err := client.Request(ctx, GET, blockPath, nil, &block)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block info: %w", err)
	}

	xBlock := xclient.NewBlock(
		client.Asset.GetChain().Chain,
		height,
		block.Hash,
		time.Unix(block.Time, 0),
	)

	blockTransactionsPaths := fmt.Sprintf("%s/txs", blockPath)
	transactionHashes := make([]string, 0)
	err = client.Request(ctx, GET, blockTransactionsPaths, nil, &transactionHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block transactions: %w", err)
	}

	return &xclient.BlockWithTransactions{
		Block:          *xBlock,
		TransactionIds: transactionHashes,
	}, nil
}

func (client *Client) Request(ctx context.Context, method string, path string, cbor []byte, resp any) error {
	apiPath := fmt.Sprintf("%s%s", API_VERSION, path)
	logger := client.Logger.WithFields(log.Fields{
		"path":   apiPath,
		"method": method,
	})
	url := fmt.Sprintf("%s/%s", client.Url, apiPath)

	logger.WithFields(log.Fields{
		"payload": hex.EncodeToString(cbor),
	}).Debug("sending request")

	var request *http.Request
	var err error
	if len(cbor) > 0 {
		request, err = http.NewRequest(method, url, bytes.NewBuffer(cbor))
		request.Header.Set("Content-Type", "application/cbor")
	} else {
		request, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if client.ProjectId != "" {
		request.Header.Set("project_id", client.ProjectId)
	}

	response, err := client.HttpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()

	buff, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send request, status: %s, error: %s", response.Status, buff)
	}

	logger.WithFields(log.Fields{
		"responseBody": string(buff),
		"status":       response.Status,
		"code":         response.StatusCode,
	}).Debug("got response")

	if len(buff) == 0 {
		return nil
	}

	err = json.Unmarshal(buff, resp)
	if err != nil {
		return fmt.Errorf("failed to decode response body: %w, buff: %s", err, string(buff))
	}

	return nil
}
