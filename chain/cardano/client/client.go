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
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	log "github.com/sirupsen/logrus"
)

const (
	FeeMargin      = 500
	TokenDecimals  = 0
	NativeDecimals = 6
	ApiVersion     = "/api/v0"
)

// Client for Template
type Client struct {
	ClientCfg           *xc.ChainClientConfig
	ChainCfg            *xc.ChainConfig
	Url                 string
	Network             string
	Logger              *log.Entry
	BlockfrostProjectId string
	HttpClient          *http.Client
}

var _ xclient.Client = &Client{}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	chainConfig := cfgI.GetChain()
	url := chainConfig.GetChain().URL
	if url == "" {
		return nil, errors.New("rpc url is empty")
	}

	network := chainConfig.GetChain().Network
	if network == "" {
		network = "mainnet"
		log.Warn("network is empty, defaulting to mainnet")
	}

	logger := log.WithFields(log.Fields{
		"chain":   chainConfig.Chain,
		"rpc":     url,
		"network": network,
	})

	return &Client{
		ClientCfg:           chainConfig.Client(),
		ChainCfg:            chainConfig,
		Url:                 url,
		Network:             network,
		Logger:              logger,
		BlockfrostProjectId: chainConfig.Auth2.LoadOrBlank(),
		HttpClient:          http.DefaultClient,
	}, nil
}

func (client *Client) FetchProtocolParameters(ctx context.Context) (types.ProtocolParameters, error) {
	var protocolParameters types.ProtocolParameters
	err := client.Get(ctx, "/epochs/latest/parameters", &protocolParameters)

	return protocolParameters, err
}

func (client *Client) FetchUtxos(ctx context.Context, address xc.Address, contract xc.ContractAddress) ([]types.Utxo, error) {
	path := fmt.Sprintf("/addresses/%s/utxos/?order=desc", string(address))
	var response []types.Utxo
	err := client.Get(ctx, path, &response)
	return response, err
}

// 1. Sort utxos by amount descending
// 2. Get minimum utxo set that can cover `targetAmount`
func GetMinUtxoSet(utxos []types.Utxo, targetAmount tx.TokenAmounts, contract xc.ContractAddress) []types.Utxo {
	slices.SortFunc(utxos, func(lhs types.Utxo, rhs types.Utxo) int {
		amountL := lhs.GetAssetAmount(contract)
		amountR := rhs.GetAssetAmount(contract)

		return amountR.Cmp(&amountL)
	})

	utxoSet := make([]types.Utxo, 0)
	amounts := tx.TokenAmounts{}
	for _, utxo := range utxos {
		for _, amount := range utxo.Amounts {
			contract := amount.Unit
			amnt := amount.Quantity
			amounts.AddAmount(xc.ContractAddress(contract), xc.NewAmountBlockchainFromStr(amnt).Uint64())
		}

		utxoSet = append(utxoSet, utxo)
		if amounts.CanCover(targetAmount) {
			break
		}
	}
	return utxoSet
}

// Create dummy Cardano transaction for fee estimation
func CreateDummyTx(args xcbuilder.TransferArgs, txInput tx_input.TxInput) (xc.Tx, error) {
	// Create a dummy transaction
	dummyTx, err := tx.NewTx(args, txInput)
	if err != nil {
		return nil, err
	}

	witness := tx.VKeyWitness{
		VKey:      make([]byte, 32),
		Signature: make([]byte, 64),
	}
	dummyTx.(*tx.Tx).Witness = &tx.Witness{
		Keys: []*tx.VKeyWitness{&witness},
	}

	return dummyTx, nil
}

// FetchTransferInput returns tx input for a Cardano transfer
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	contract, ok := args.GetContract()
	if !ok {
		contract = types.Lovelace
	}

	utxos, err := client.FetchUtxos(ctx, args.GetFrom(), contract)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch utxos: %w", err)
	}

	gasBudget := client.ClientCfg.GasBudgetDefault.ToBlockchain(NativeDecimals).Uint64()
	targetAmounts := tx.TokenAmounts{}
	targetAmounts.AddAmount(types.Lovelace, gasBudget)
	targetAmounts.AddAmount(contract, args.GetAmount().Uint64())
	utxos = GetMinUtxoSet(utxos, targetAmounts, contract)

	var latestBlock types.Block
	err = client.Get(ctx, "/blocks/latest", &latestBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block info: %w", err)
	}

	protocolParams, err := client.FetchProtocolParameters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch protocol parameters: %w", err)
	}

	transactionActiveTime := uint64(client.ClientCfg.TransactionActiveTime.Seconds())
	txInput := tx_input.TxInput{
		Utxos:                   utxos,
		Slot:                    latestBlock.Slot,
		Fee:                     0,
		TransactionValidityTime: transactionActiveTime,
	}
	dummyTx, err := CreateDummyTx(args, txInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create fee estimation transaction: %w", err)
	}
	cbor, err := dummyTx.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize fee stimation transaction: %w", err)
	}
	txSize := len(cbor)
	txInput.Fee = protocolParams.FeePerByte*uint64(txSize) + protocolParams.FixedFee + FeeMargin

	return &txInput, nil
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
	err = client.Post(ctx, "/tx/submit", bytes, &response)
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
	err := client.Get(ctx, "/blocks/latest", &latestBlock)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch latest block: %w", err)
	}

	var transactionInfo types.TransactionInfo
	transactionPath := fmt.Sprintf("/txs/%s", string(txHash))
	err = client.Get(ctx, transactionPath, &transactionInfo)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch transaction info: %w", err)
	}

	var blockInfo types.Block
	blockPath := fmt.Sprintf("/blocks/%d", transactionInfo.BlockHeight)
	err = client.Get(ctx, blockPath, &blockInfo)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch block info: %w", err)
	}

	chain := client.ChainCfg.Chain
	timestamp := time.Unix(blockInfo.Time, 0)
	block := xclient.NewBlock(
		chain,
		uint64(transactionInfo.BlockHeight),
		blockInfo.Hash,
		timestamp,
	)

	txInfo := xclient.NewTxInfo(
		block,
		client.ChainCfg,
		string(txHash),
		blockInfo.Confirmations,
		nil,
	)

	var transactionUtxos types.TransactionUtxos
	transactionUtxosPath := fmt.Sprintf("%s/utxos", transactionPath)
	err = client.Get(ctx, transactionUtxosPath, &transactionUtxos)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch transaction utxos: %w", err)
	}
	contractToMovement := make(map[xc.ContractAddress]*xclient.Movement)
	for _, input := range transactionUtxos.Inputs {
		addr := xc.Address(input.Address)
		for _, amount := range input.Amounts {
			contract := xc.ContractAddress(amount.Unit)
			if contract == types.Lovelace {
				contract = types.Ada
			}
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
			if contract == types.Lovelace {
				contract = types.Ada
			}
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

	decimals, err := client.FetchDecimals(ctx, types.Ada)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch decimals: %w", err)
	}

	feeAmount := xc.NewAmountBlockchainFromStr(transactionInfo.Fees)
	txInfo.Fees = []*xclient.Balance{
		xclient.NewBalance(xc.ADA, types.Ada, feeAmount, &decimals),
	}
	txInfo.Final = int(txInfo.Confirmations) > client.ChainCfg.ConfirmationsFinal

	return *txInfo, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	path := fmt.Sprintf("/addresses/%s", string(args.Address()))
	var getAddressInfoResponse types.GetAddressInfoResponse
	err := client.Get(ctx, path, &getAddressInfoResponse)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to fetch address info: %w", err)
	}

	contract, ok := args.Contract()
	if !ok {
		contract = types.Lovelace
	}

	for _, amount := range getAddressInfoResponse.Amounts {
		if amount.Unit == string(contract) {
			return xc.NewAmountBlockchainFromStr(amount.Quantity), nil
		}
	}

	return xc.NewAmountBlockchainFromUint64(0), nil
}

// types.Ada uses 6 decimals, and tokens use 0 decimals.
// Token decimals are tricky at the moment, so we return 0 for now.
func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if contract == types.Lovelace || contract == "" || contract == types.Ada {
		return NativeDecimals, nil
	}
	return TokenDecimals, nil
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
	err := client.Get(ctx, blockPath, &block)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block info: %w", err)
	}

	xBlock := xclient.NewBlock(
		client.ChainCfg.Chain,
		height,
		block.Hash,
		time.Unix(block.Time, 0),
	)

	blockTransactionsPaths := fmt.Sprintf("%s/txs", blockPath)
	transactionHashes := make([]string, 0)
	err = client.Get(ctx, blockTransactionsPaths, &transactionHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block transactions: %w", err)
	}

	return &xclient.BlockWithTransactions{
		Block:          *xBlock,
		TransactionIds: transactionHashes,
	}, nil
}

func (client *Client) request(ctx context.Context, method string, path string, cbor []byte, resp any) error {
	apiPath := fmt.Sprintf("%s%s", ApiVersion, path)
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

	if client.BlockfrostProjectId != "" {
		request.Header.Set("project_id", client.BlockfrostProjectId)
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
		var errorResponse types.Error
		err = json.Unmarshal(buff, &errorResponse)
		if err != nil {
			return fmt.Errorf("failed to decode error body: %w, buff: %s", err, string(buff))
		}
		return &errorResponse
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

func (client *Client) Get(ctx context.Context, path string, resp any) error {
	return client.request(ctx, "GET", path, nil, resp)
}

func (client *Client) Post(ctx context.Context, path string, cbor []byte, resp any) error {
	return client.request(ctx, "POST", path, cbor, resp)
}
