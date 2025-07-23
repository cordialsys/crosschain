package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	icpaddress "github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/agent"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	icptx "github.com/cordialsys/crosschain/chain/internet_computer/tx"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	log "github.com/sirupsen/logrus"
)

// Client for InternetComputerProtocol
type Client struct {
	Agent  *agent.Agent
	Asset  xc.ITask
	Logger *log.Entry
	Url    *url.URL
}

var _ xclient.Client = &Client{}

var submitedTxs map[xc.TxHash]uint64 = make(map[xc.TxHash]uint64)

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
	config := agent.AgentConfig{
		Identity:      icpaddress.Ed25519Identity{},
		IngressExpiry: 0,
		Url:           url,
		Logger:        logger,
	}
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
	canister := types.IcpLedgerPrincipal
	if contract != "" {
		c, err := address.Decode(string(contract))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to decode contract: %w", err)
		}
		canister = c
	}

	var fee idl.Nat
	err := client.Agent.Query(
		canister, types.MethodICRCFee, []any{}, []any{&fee},
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
		contract = xc.ContractAddress(types.IcpLedgerPrincipal.String())
	}
	memo, hasMemo := args.GetMemo()

	fee, err := client.fetchFee(ctx, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fee: %w", err)
	}

	txInput := tx_input.NewTxInput()
	txInput.Fee = fee.Uint64()
	txInput.CreatedAtTime = uint64(time.Now().UnixNano())
	txInput.Canister = contract

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
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a InternetComputerProtocol tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	icpTx, ok := tx.(*icptx.Tx)
	if !ok {
		return errors.New("invalid transaction type")
	}

	var result types.TransferResult
	err := icpTx.Agent.Call(icpTx.Request.CanisterID, icpTx.Request.RequestID(), icpTx.SignedRequest, []any{&result})
	if err != nil {
		return fmt.Errorf("failed to submit tx: %w", err)
	}

	if result.Err != nil {
		return fmt.Errorf("failed to submit tx: %v", result.Err)
	} else if result.Ok != nil {
		hash := tx.Hash()
		submitedTxs[hash] = *result.Ok
	}
	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	return xclient.LegacyTxInfo{}, errors.New("deprecated")
}

func tryGetSubmitedTxBlockIndex(txHash xc.TxHash) (uint64, bool) {
	index, ok := submitedTxs[txHash]
	if ok {
		return index, ok
	}

	return 0, false
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	blockIndex, ok := tryGetSubmitedTxBlockIndex(txHash)
	if !ok {
		index, err := strconv.Atoi(string(txHash))
		if err != nil {
			return xclient.TxInfo{}, fmt.Errorf("tx info can be fetched only by block index, got: %s instead, err: %w", txHash, err)
		}
		blockIndex = uint64(index)

	}
	block, err := client.fetchRawBlock(ctx, blockIndex)
	if err != nil {
		return xclient.TxInfo{}, fmt.Errorf("failed to fetch block: %w", err)
	}

	blockHash, err := block.Hash()
	if err != nil {
		return xclient.TxInfo{}, err
	}

	transactionHash, err := block.Transaction.Hash()
	if err != nil {
		return xclient.TxInfo{}, err
	}

	height, err := client.fetchHeight(ctx)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	xBlock := xclient.NewBlock(xc.ICP, blockIndex, blockHash, block.Timestamp.ToUnixTime())
	txInfo := xclient.NewTxInfo(xBlock, client.Asset.GetChain(), transactionHash, height-blockIndex, nil)

	sourceAddress := xc.Address(block.Transaction.SourceAddress())
	destinationAddress := xc.Address(block.Transaction.DestinationAddress())
	amount := block.Transaction.Amount()
	xcAmount := xc.NewAmountBlockchainFromUint64(amount.E8s)

	movement := xclient.NewMovement(client.Asset.GetChain().Chain, "")
	movement.AddSource(sourceAddress, xcAmount, nil)
	movement.AddDestination(destinationAddress, xcAmount, nil)
	movement.SetMemo(fmt.Sprintf("%d", block.Transaction.Memo))

	txInfo.AddMovement(movement)

	fee := xc.NewAmountBlockchainFromUint64(block.Transaction.Fee().E8s)
	txInfo.AddFee(sourceAddress, "", fee, nil)
	txInfo.Fees = txInfo.CalculateFees()
	txInfo.Final = int(txInfo.Confirmations) > client.Asset.GetChain().ConfirmationsFinal

	return *txInfo, nil
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

		account := types.ICRC1Account{
			Owner: owner,
		}

		var balance idl.Nat

		err = client.Agent.Query(icrcCanister, types.MethodICRCBalanceOf, []any{account}, []any{&balance})
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to query balance: %w", err)
		}

		return xc.NewAmountBlockchainFromUint64(balance.BigInt().Uint64()), err
	} else {
		accountID, err := hex.DecodeString(string(args.Address()))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to decode address: %w", err)
		}

		var icpBalance types.IcpBalance
		err = client.Agent.Query(types.IcpLedgerPrincipal, types.MethodAccountBalance, []any{
			types.BalanceArgs{Account: accountID},
		}, []any{&icpBalance})

		return xc.NewAmountBlockchainFromUint64(icpBalance.E8S), err
	}
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return 8, nil
}

func (client *Client) fetchRawBlock(ctx context.Context, blockIndex uint64) (types.Block, error) {
	var response types.QueryBlocksResponse
	err := client.Agent.Query(types.IcpLedgerPrincipal, types.MethodQueryBlocks,
		[]any{types.QueryBlocksArgs{
			Start:  blockIndex,
			Length: 1,
		}},
		[]any{&response},
	)
	if err != nil {
		return types.Block{}, fmt.Errorf("failed to query blocks: %w", err)
	}

	if len(response.Blocks) == 1 {
		return response.Blocks[0], nil
	}

	// Query archive canister if block is archived
	if len(response.ArchivedBlocks) == 1 {
		targetArchive := response.ArchivedBlocks[0]
		var archiveResponse types.GetBlocksResult
		err = client.Agent.Query(
			targetArchive.Callback.Method.Principal,
			targetArchive.Callback.Method.Method,
			[]any{
				types.QueryBlocksArgs{
					Start:  targetArchive.Start,
					Length: targetArchive.Length,
				},
			},
			[]any{&archiveResponse},
		)
		if err != nil {
			return types.Block{}, fmt.Errorf("failed to query archive canister: %w", err)
		}
		if archiveResponse.Ok == nil {
			return types.Block{}, fmt.Errorf("archive canister error: %v", *archiveResponse.Err)
		} else {
			return archiveResponse.Ok.Blocks[0], nil
		}
	}

	fmt.Printf("Raw response: %+v", response)

	return types.Block{}, errors.New("failed to fetch block")
}

func (client *Client) fetchHeight(ctx context.Context) (uint64, error) {
	var response types.QueryBlocksResponse
	err := client.Agent.Query(types.IcpLedgerPrincipal, types.MethodQueryBlocks,
		// Start and Lenght 0 to query block height
		[]any{types.QueryBlocksArgs{
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

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	// Fetch latest ledger index if `height` is not specified
	height, ok := args.Height()
	if !ok {
		h, err := client.fetchHeight(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query block height: %w", err)
		}

		height = h
	}

	block, err := client.fetchRawBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	hash, err := block.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate block hash: %w", err)
	}
	timestamp := block.Timestamp.ToUnixTime()
	xcBlock := xclient.NewBlock(xc.ICP, height, hash, timestamp)

	transactions := make([]string, 0, 1)
	txHash, err := block.Transaction.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate transaction hash: %w", err)
	}
	transactions = append(transactions, txHash)
	return &xclient.BlockWithTransactions{
		Block:          *xcBlock,
		TransactionIds: transactions,
		SubBlocks:      []*xclient.SubBlockWithTransactions{},
	}, nil
}
