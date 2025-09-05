package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	icpaddress "github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/agent"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icp"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icrc"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	log "github.com/sirupsen/logrus"
)

const NumberOfTransactionsForTxInfo = uint64(100)

// Client for InternetComputerProtocol
type Client struct {
	Agent  *agent.Agent
	Asset  xc.ITask
	Logger *log.Entry
	Url    *url.URL
}

var _ xclient.Client = &Client{}

// Not all ledger canisters support `icrc106_get_index_principal` method
var indexCanisters map[string]icpaddress.Principal = map[string]icpaddress.Principal{
	icp.LedgerPrincipal.String():  icpaddress.MustDecode("qhbym-qaaaa-aaaaa-aaafq-cai"),
	"ly36x-wiaaa-aaaai-aqj7q-cai": icpaddress.MustDecode("4cx56-naaaa-aaaai-aqkaa-cai"), // VCHF
	"wu6g4-6qaaa-aaaan-qmrza-cai": icpaddress.MustDecode("wi24n-jqaaa-aaaan-qmr3a-cai"), // VEUR
}

func newAgentConfig(identity icpaddress.Ed25519Identity, remote *url.URL, logger *log.Entry) agent.AgentConfig {
	agentConfig := agent.NewAgentConfig()
	agentConfig.SetIdentity(identity)
	agentConfig.SetUrl(remote)
	agentConfig.SetLogger(logger)
	agentConfig.SetIngressExpiry(2 * time.Minute)
	return agentConfig
}

// NewClient returns a new InternetComputerProtocol Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	url, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}
	logger := log.WithFields(log.Fields{
		"chain":   cfg.Chain,
		"rpc":     cfg.URL,
		"network": cfg.Network,
	})
	config := newAgentConfig(icpaddress.Ed25519Identity{}, url, logger)
	agent, err := agent.NewAgent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ICP agent: %w", err)
	}

	return &Client{
		Agent:  agent,
		Asset:  cfgI,
		Logger: logger,
		Url:    url,
	}, nil
}

func (client *Client) fetchFee(ctx context.Context, contract xc.ContractAddress) (xc.AmountBlockchain, error) {
	canister := icp.LedgerPrincipal
	if contract != "" {
		c, err := icpaddress.Decode(string(contract))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to decode contract: %w", err)
		}
		canister = c
	}

	var fee idl.Nat
	err := client.Agent.Query(
		canister, icrc.MethodFee, []any{}, []any{&fee},
	)
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to query icrc fee: %w", err)
	}

	return xc.NewAmountBlockchainFromUint64(fee.BigInt().Uint64()), nil
}

// FetchTransferInput returns tx input for a InternetComputerProtocol tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	contract, isIcrc := args.GetContract()
	if !isIcrc {
		contract = xc.ContractAddress(icp.LedgerPrincipal.String())
	}
	memo, hasMemo := args.GetMemo()

	fee, err := client.fetchFee(ctx, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fee: %w", err)
	}

	txInput := tx_input.NewTxInput()
	txInput.Fee = fee.Uint64()
	txInput.CreateTime = time.Now().Unix()
	txInput.Canister = contract
	randomNonce := make([]byte, 10)
	_, err = rand.Read(randomNonce)
	if err != nil {
		return txInput, err
	}
	txInput.Nonce = hex.EncodeToString(randomNonce)

	if hasMemo {
		if isIcrc {
			icrcMemo := []byte(memo)
			txInput.ICRC1Memo = &icrcMemo
		} else {
			icpMemo, err := strconv.Atoi(memo)
			if err != nil {
				return txInput, fmt.Errorf("icp ledger supports only uint64 memos: %w", err)
			}
			txInput.Memo = uint64(icpMemo)
		}
	}

	return txInput, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a InternetComputerProtocol tx
func (client *Client) SubmitTx(ctx context.Context, txI xc.Tx) error {
	withMetadata, ok := txI.(xc.TxWithMetadata)
	if !ok {
		return fmt.Errorf("ICP transactions must implement TxWithMetadata")
	}
	serializedSignedTx, err := txI.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}
	metadataBz, err := withMetadata.GetMetadata()
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}
	var metadata tx.BroadcastMetadata
	err = json.Unmarshal(metadataBz, &metadata)
	if err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}
	requestIdHex := metadata.RequestID
	requestId, err := hex.DecodeString(requestIdHex)
	if err != nil {
		return fmt.Errorf("failed to decode request id: %w", err)
	}

	identity := icpaddress.NewEd25519Identity(metadata.SenderPublicKey)
	agentConfig := newAgentConfig(identity, client.Url, client.Logger)

	agent, err := agent.NewAgent(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	canister, err := icpaddress.Decode(metadata.CanisterID)
	if err != nil {
		return fmt.Errorf("failed to decode canister principal: %w", err)
	}

	if metadata.IsIcrcTx {
		return client.CallIcrcTransaction(agent, types.RequestID(requestId), canister, serializedSignedTx)
	} else {
		return client.CallIcpTransaction(agent, types.RequestID(requestId), canister, serializedSignedTx)
	}
}

func (client *Client) CallIcpTransaction(a *agent.Agent, id types.RequestID, canister icpaddress.Principal, tx []byte) error {
	var result icp.TransferResult
	err := a.Call(canister, id, tx, []any{&result})
	if err != nil {
		return fmt.Errorf("failed to submit tx: %w", err)
	}

	if result.Err != nil {
		return fmt.Errorf("tx rejected: %v", result.Err)
	}

	return nil
}

func (client *Client) CallIcrcTransaction(a *agent.Agent, id types.RequestID, canister icpaddress.Principal, tx []byte) error {
	var result icrc.TransferResult
	err := a.Call(canister, id, tx, []any{&result})
	if err != nil {
		return fmt.Errorf("failed to submit tx: %w", err)
	}

	if result.Err != nil {
		return fmt.Errorf("tx rejected: %v", result.Err)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, fmt.Errorf("deprecated")
}

func (client *Client) fetchIndexPrincipal(ctx context.Context, canister icpaddress.Principal) (icpaddress.Principal, error) {
	var response icrc.GetIndexPrincipalResponse
	err := client.Agent.Query(
		canister,
		icrc.MethodGetIndexPrincipal,
		[]any{},
		[]any{&response},
	)
	if err != nil {
		return icpaddress.Principal{}, fmt.Errorf("failed to query index principal: %w", err)
	}
	if response.Err != nil {
		return icpaddress.Principal{}, fmt.Errorf("canister error: %w", response.Err)
	}

	return *response.Ok, nil
}

func (client *Client) fetchAccountTransactions(ctx context.Context, sender xc.Address, canister icpaddress.Principal) ([]types.TransactionWithId, error) {
	// Use get_account_transactions for ICRC addresses
	icrcAccount, err := icrc.DecodeAccount(string(sender))
	if err == nil {
		args := icrc.GetAccountTransactionsArgs{
			MaxResults: idl.NewNat(NumberOfTransactionsForTxInfo),
			Account:    icrcAccount,
		}
		var response icrc.GetAccountTransactionsResponse
		err := client.Agent.Query(
			canister,
			icrc.MethodGetAccountTransactions,
			[]any{args},
			[]any{&response},
		)

		if err != nil {
			return nil, fmt.Errorf("failed to query account transactions: %w", err)
		}

		if response.Error != nil {
			return nil, fmt.Errorf("canister error: %s", response.Error.Message)
		}

		transactions := make([]types.TransactionWithId, 0)
		for _, tx := range response.Ok.Transactions {
			transactions = append(transactions, types.TransactionWithId{
				Transaction: tx.Transaction,
				Id:          tx.Id.BigInt().Uint64(),
			})
		}

		return transactions, err
	}

	// Try get_account_identifier_transactions for ICP addresses
	args := icp.GetAccountIdentifierTransactions{
		MaxResults:        NumberOfTransactionsForTxInfo,
		Start:             nil,
		AccountIdentifier: string(sender),
	}
	var response icp.GetAccountIdentifierTransactionsResult
	err = client.Agent.Query(
		canister,
		icp.MethodGetAccountIdentifierTransactions,
		[]any{args},
		[]any{&response},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query account transactions: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("canister error: %s", response.Error.Message)
	}

	transactions := make([]types.TransactionWithId, 0)
	for _, tx := range response.Ok.Transactions {
		transactions = append(transactions, types.TransactionWithId{
			Transaction: tx.Transaction,
			Id:          tx.Id.BigInt().Uint64(),
		})
	}

	return transactions, err
}

func (client *Client) fetchAssetName(ctx context.Context, canister icpaddress.Principal) (string, error) {
	var response string
	err := client.Agent.Query(
		canister,
		icrc.MethodName,
		[]any{},
		[]any{&response},
	)

	return response, err
}

func (client *Client) fetchTxInfoByBlockIndex(ctx context.Context, canister icpaddress.Principal, blockIndex uint64) (xclient.TxInfo, error) {
	block, err := client.fetchRawBlock(ctx, canister, blockIndex)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch block: %w", err)
	}

	blockHash, err := block.Hash()
	if err != nil {
		return xclient.TxInfo{}, err
	}

	height, err := client.fetchHeight(ctx, canister)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	ts, err := block.Timestamp()
	if err != nil {
		return xclient.TxInfo{}, err
	}
	blockTimestamp := time.Unix(0, int64(ts)).UTC()
	xBlock := xclient.NewBlock(xc.ICP, blockIndex, blockHash, blockTimestamp)

	transactionHash := types.GetTransacionHash(blockIndex, canister)
	txInfo := xclient.NewTxInfo(xBlock, client.Asset.GetChain(), transactionHash, height-blockIndex, nil)

	transaction, err := block.Transaction()
	if err != nil {
		return xclient.TxInfo{}, err
	}

	sourceAddress := xc.Address(transaction.SourceAddress())
	destinationAddress := xc.Address(transaction.DestinationAddress())
	amount, err := transaction.Amount()
	if err != nil {
		return xclient.TxInfo{}, err
	}
	xcAmount := xc.NewAmountBlockchainFromUint64(amount)

	contract := xc.ContractAddress("")
	if canister.Encode() != icp.LedgerPrincipal.Encode() {
		contract = xc.ContractAddress(canister.Encode())
	}
	movement := xclient.NewMovement(client.Asset.GetChain().Chain, contract)
	movement.AddSource(sourceAddress, xcAmount, nil)
	movement.AddDestination(destinationAddress, xcAmount, nil)
	movement.SetMemo(transaction.Memo())

	txInfo.AddMovement(movement)

	fee := transaction.Fee()
	if fee == 0 {
		fee = block.Fee()
	}

	xcFee := xc.NewAmountBlockchainFromUint64(fee)
	txInfo.AddFee(sourceAddress, contract, xcFee, nil)
	txInfo.Fees = txInfo.CalculateFees()
	txInfo.Final = int(txInfo.Confirmations) > client.Asset.GetChain().ConfirmationsFinal

	return *txInfo, nil
}

// tryFetchTxInfoByHash attempts to retrieve transaction block index by hash from an ICP/ICRC ledger:
// - fetch index canister address
// - fetch last N account transactions
// - check if there is a transaction with hash == requested hash
// - proceed with 'client.fetchTxInfoByBlockIndex' if matching transaction hash was found
func (client *Client) tryFetchTxInfoByHash(ctx context.Context, ledgerCanister icpaddress.Principal, txHash xc.TxHash, sender xc.Address) (xclient.TxInfo, error) {
	indexCanister, ok := indexCanisters[ledgerCanister.String()]
	if !ok {
		indexer, err := client.fetchIndexPrincipal(ctx, ledgerCanister)
		if err != nil {
			return xclient.TxInfo{}, fmt.Errorf("failed to fetch index principal: %w", err)
		}

		indexCanister = indexer
	}

	transactions, err := client.fetchAccountTransactions(ctx, sender, indexCanister)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch account transactions: %w", err)
	}

	for _, tx := range transactions {
		th, err := tx.Transaction.Hash()
		if err != nil {
			return xclient.TxInfo{}, fmt.Errorf("failed to compute tx hash: %w", err)
		}

		if th == string(txHash) {
			blockHeight := tx.Id
			return client.fetchTxInfoByBlockIndex(ctx, ledgerCanister, blockHeight)
		}
	}
	return xclient.TxInfo{}, xcerrors.TransactionNotFoundf("no matching transaction found in recent account history")
}

func getBlockAndContractIndex(args *txinfo.Args) (uint64, icpaddress.Principal, bool, error) {
	ledgerCanister := icp.LedgerPrincipal
	if contract, ok := args.Contract(); ok {
		c, err := icpaddress.Decode(string(contract))
		if err != nil {
			return 0, ledgerCanister, false, fmt.Errorf("invalid canister contract: %w", err)
		}
		ledgerCanister = c
	}
	if blockHeight, ok := args.BlockHeight(); ok {
		return blockHeight.Uint64(), ledgerCanister, true, nil
	}

	// Check if txHash is in "blockheight.contract" format
	// Overwrite `args.Contract()` in this case
	parts := strings.Split(string(args.TxHash()), ".")
	if len(parts) == 2 {
		blockHeight, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, ledgerCanister, false, fmt.Errorf("failed to parse block height: %w", err)
		}

		ledgerCanister, err := icpaddress.Decode(parts[1])
		if err != nil {
			return 0, ledgerCanister, false, fmt.Errorf("failed to decode ledger canister: %w", err)
		}

		return uint64(blockHeight), ledgerCanister, true, nil
	}

	return 0, ledgerCanister, false, nil
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (xclient.TxInfo, error) {

	// We can fetch the transaction direclty if we receive a block index
	// Fallback to account history lookup otherwise
	block, ledgerCanister, ok, err := getBlockAndContractIndex(args)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	if ok {
		return client.fetchTxInfoByBlockIndex(ctx, ledgerCanister, block)
	} else {
		// fallback to account history lookup
		senderAddress, ok := args.Sender()
		if !ok {
			return xclient.TxInfo{}, fmt.Errorf("must use block-height to lookup or specify sender address")
		}
		hash := args.TxHash()
		return client.tryFetchTxInfoByHash(ctx, ledgerCanister, hash, senderAddress)
	}
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	contract, ok := args.Contract()
	if ok {
		icrcCanister, err := icpaddress.Decode(string(contract))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to decoede icrc canister principal: %w", err)
		}

		owner, err := icpaddress.Decode(string(args.Address()))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to decoede owner principal: %w", err)
		}

		account := icrc.Account{
			Owner: owner,
		}

		var balance idl.Nat

		err = client.Agent.Query(icrcCanister, icrc.MethodBalanceOf, []any{account}, []any{&balance})
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to query balance: %w", err)
		}

		return xc.NewAmountBlockchainFromUint64(balance.BigInt().Uint64()), err
	} else {
		accountID, err := hex.DecodeString(string(args.Address()))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to decode address: %w", err)
		}

		var icpBalance icp.Balance
		err = client.Agent.Query(icp.LedgerPrincipal, icp.MethodAccountBalance, []any{
			icp.GetBalanceArgs{Account: accountID},
		}, []any{&icpBalance})

		return xc.NewAmountBlockchainFromUint64(icpBalance.E8S), err
	}
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if contract == "" {
		return 8, nil
	}

	canister, err := icpaddress.Decode(string(contract))
	if err != nil {
		return 0, fmt.Errorf("failed to decode canister principal: %w", err)
	}

	var metadata icrc.MapWrapper
	err = client.Agent.Query(canister, icrc.MethodMetadata, []any{}, []any{&metadata})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	var decimals idl.Nat
	ok, err := metadata.GetValue(icrc.KeyDecimals, &decimals)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch decimals: %w", err)
	}
	if !ok {
		return 8, nil
	}

	return int(decimals.BigInt().Uint64()), nil
}

func (client *Client) fetchHeight(ctx context.Context, canister icpaddress.Principal) (uint64, error) {
	if canister.String() == icp.LedgerPrincipal.String() {
		return client.fetchIcpHeight(ctx)
	} else {
		return client.fetchIcrcHeight(ctx, canister)
	}
}

func (client *Client) fetchIcpHeight(ctx context.Context) (uint64, error) {
	var response icp.QueryBlocksResponse
	err := client.Agent.Query(icp.LedgerPrincipal, icp.MethodQueryBlocks,
		// Start and Lenght 0 to query block height
		[]any{icp.QueryBlocksArgs{
			Start:  0,
			Length: 0,
		}},
		[]any{&response},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query block height: %w", err)
	}

	// Sometimes `ChainLenght` block cannot be fetched
	return response.ChainLength - 1, nil
}

func (client *Client) fetchRawIcpBlock(ctx context.Context, blockIndex uint64) (icp.Block, error) {
	var response icp.QueryBlocksResponse
	err := client.Agent.Query(icp.LedgerPrincipal, icp.MethodQueryBlocks,
		[]any{icp.QueryBlocksArgs{
			Start:  blockIndex,
			Length: 1,
		}},
		[]any{&response},
	)
	if err != nil {
		return icp.Block{}, fmt.Errorf("failed to query blocks: %w", err)
	}

	if len(response.Blocks) == 1 {
		return response.Blocks[0], nil
	}

	// Query archive canister if block is archived
	if len(response.ArchivedBlocks) == 1 {
		targetArchive := response.ArchivedBlocks[0]
		var archiveResponse icp.GetBlocksResult
		err = client.Agent.Query(
			targetArchive.Callback.Method.Principal,
			targetArchive.Callback.Method.Method,
			[]any{
				icp.QueryBlocksArgs{
					Start:  targetArchive.Start,
					Length: targetArchive.Length,
				},
			},
			[]any{&archiveResponse},
		)
		if err != nil {
			return icp.Block{}, fmt.Errorf("failed to query archive canister: %w", err)
		}
		if archiveResponse.Ok == nil {
			return icp.Block{}, fmt.Errorf("archive canister error: %v", *archiveResponse.Err)
		} else {
			return archiveResponse.Ok.Blocks[0], nil
		}
	}

	return icp.Block{}, fmt.Errorf("failed to fetch block")
}

func (client *Client) fetchRawIcrcBlock(ctx context.Context, canister icpaddress.Principal, blockIndex uint64) (icrc.Block, error) {
	var response icrc.GetBlocksResponse
	err := client.Agent.Query(
		canister,
		icrc.MethodGetBlocks,
		[]any{[]icrc.GetBlocksRequest{
			{
				Start:  idl.NewNat(uint64(blockIndex)),
				Length: idl.NewNat(uint64(1)),
			},
		}},
		[]any{&response},
	)
	if err != nil {
		return icrc.Block{}, fmt.Errorf("failed to fetch icrc block: %w", err)
	}

	currentHeight := response.GetHeight()
	if currentHeight < blockIndex {
		return icrc.Block{}, fmt.Errorf("invalid block(%d), current chain length: %d", blockIndex, currentHeight)
	}

	if len(response.Blocks) == 1 {
		return response.Blocks[0].Block, nil
	}

	if len(response.ArchivedBlocks) == 1 {
		archive := response.ArchivedBlocks[0]
		var archiveResponse icrc.GetBlocksResponse
		err = client.Agent.Query(
			archive.Callback.Method.Principal,
			archive.Callback.Method.Method,
			[]any{archive.Args},
			[]any{&archiveResponse},
		)
		if err != nil {
			return icrc.Block{}, fmt.Errorf("failed to query archive canister: %w", err)
		}

		if len(archiveResponse.Blocks) == 1 {
			return archiveResponse.Blocks[0].Block, nil
		}
	}
	return icrc.Block{}, errors.New("coudln't find block")
}

func (client *Client) fetchRawBlock(ctx context.Context, canister icpaddress.Principal, blockIndex uint64) (types.Block, error) {
	if canister.String() == icp.LedgerPrincipal.String() {
		return client.fetchRawIcpBlock(ctx, blockIndex)
	} else {
		return client.fetchRawIcrcBlock(ctx, canister, blockIndex)
	}

}

func (client *Client) fetchIcrcHeight(ctx context.Context, canister icpaddress.Principal) (uint64, error) {
	var response icrc.GetBlocksResponse
	err := client.Agent.Query(
		canister,
		icrc.MethodGetBlocks,
		[]any{[]icrc.GetBlocksRequest{{
			Start:  idl.Nat{},
			Length: idl.Nat{},
		}}},
		[]any{&response},
	)

	if err != nil {
		return 0, fmt.Errorf("failed to query block height: %w", err)
	}

	return response.GetHeight() - 1, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	contract, ok := args.Contract()
	canister := icp.LedgerPrincipal
	if ok {
		c, err := icpaddress.Decode(string(contract))
		if err != nil {
			return nil, fmt.Errorf("failed to decode canister principal: %w", err)
		}
		canister = c
	}

	// Fetch latest ledger index if `height` is not specified
	height, ok := args.Height()
	if !ok {
		h, err := client.fetchHeight(ctx, canister)
		if err != nil {
			return nil, fmt.Errorf("failed to query block height: %w", err)
		}

		height = h
	}

	block, err := client.fetchRawBlock(ctx, canister, height)
	if err != nil {
		return nil, err
	}

	hash, err := block.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate block hash: %w", err)
	}

	ts, err := block.Timestamp()
	if err != nil {
		return nil, fmt.Errorf("failed to get block timestamp: %w", err)
	}
	timestamp := time.Unix(0, int64(ts)).UTC()
	xcBlock := xclient.NewBlock(xc.ICP, height, hash, timestamp)

	transactions := make([]string, 0, 1)
	txHash := types.GetTransacionHash(height, canister)

	transactions = append(transactions, txHash)
	return &xclient.BlockWithTransactions{
		Block:          *xcBlock,
		TransactionIds: transactions,
		SubBlocks:      []*xclient.SubBlockWithTransactions{},
	}, nil
}
