package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	core "github.com/okx/go-wallet-sdk/coins/tron/pb"
	"github.com/okx/go-wallet-sdk/crypto/base58"
	"github.com/sirupsen/logrus"
)

var _ xclient.Client = &Client{}

const TRANSFER_EVENT_HASH_HEX = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const TX_TIMEOUT = 2 * time.Hour

// Client for Template
type Client struct {
	// client *client.GrpcClient
	client *httpclient.Client

	contract xc.ContractAddress
	chain    *xc.ChainConfig
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

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
	input.Expiration = unix + int64((TX_TIMEOUT).Seconds())
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// tron uses recent-block-hash like mechanism like solana, but with explicit timestamps
	return true
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	for _, other := range others {
		oldInput, ok := other.(*TxInput)
		if ok {
			if input.Timestamp <= oldInput.Expiration {
				return false
			}
		} else {
			// can't tell (this shouldn't happen) - default false
			return false
		}
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

	client, err := httpclient.NewHttpClient(cfg.URL)
	// client := client.NewGrpcClient(cfg.URL)
	// err := client.Start(grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		client,
		xc.ContractAddress(cfgI.GetContract()),
		cfg,
	}, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := new(TxInput)

	// Getting blockhash details from the CreateTransfer endpoint as TRON uses an unusual hashing algorithm (SHA2256SM3), so we can't do a minimal
	// retrieval and just get the blockheaders
	dummyTx, err := client.client.CreateTransaction(string(args.GetFrom()), string(args.GetTo()), 5)
	if err != nil {
		return nil, err
	}

	input.RefBlockBytes = dummyTx.RawData.RefBlockBytes
	input.RefBlockHash = dummyTx.RawData.RefBlockHashBytes
	// set timeout period
	input.Timestamp = time.Now().Unix()
	input.Expiration = time.Now().Add(TX_TIMEOUT).Unix()

	return input, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Tron tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	t := tx.(*Tx)
	bz, err := t.Serialize()
	if err != nil {
		return err
	}

	_, err = client.client.BroadcastHex(hex.EncodeToString(bz))

	return err
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	tx, err := client.client.GetTransactionByID(string(txHash))
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	info, err := client.client.GetTransactionInfoByID(string(txHash))
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	block, err := client.client.GetBlockByNum(info.BlockNumber)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	var from xc.Address
	var to xc.Address
	var amount xc.AmountBlockchain
	sources, destinations := deserialiseTransactionEvents(info.Logs)
	// If we cannot retrieve transaction events, we can infer that the TX is a native transfer
	if len(sources) == 0 && len(destinations) == 0 {
		from, to, amount, err = deserialiseNativeTransfer(tx)
		if err != nil {
			logrus.WithError(err).Warn("unknown transaction")
		} else {
			source := new(xc.LegacyTxInfoEndpoint)
			source.Address = from
			source.Amount = amount
			source.Asset = string(client.chain.Chain)
			source.NativeAsset = client.chain.Chain

			destination := new(xc.LegacyTxInfoEndpoint)
			destination.Address = to
			destination.Amount = amount
			destination.Asset = string(client.chain.Chain)
			destination.NativeAsset = client.chain.Chain

			sources = append(sources, source)
			destinations = append(destinations, destination)
		}
	}

	txInfo := xc.LegacyTxInfo{
		BlockHash:       block.BlockId,
		TxID:            string(txHash),
		From:            from,
		To:              to,
		ContractAddress: xc.ContractAddress(info.ContractAddress),
		Amount:          amount,
		Fee:             xc.NewAmountBlockchainFromUint64(uint64(info.Fee)),
		BlockIndex:      int64(info.BlockNumber),
		BlockTime:       int64(info.BlockTimeStamp / 1000),
		Confirmations:   0,
		Status:          xc.TxStatusSuccess,
		Sources:         sources,
		Destinations:    destinations,
		Time:            int64(info.BlockTimeStamp),
		TimeReceived:    0,
		Error:           "",
	}
	if info.Receipt.Result == httpclient.Revert {
		txInfo.Error = "transaction reverted"
		txInfo.Sources = nil
		txInfo.Destinations = nil
	}

	return txInfo, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(client.chain, legacyTx, xclient.Account), nil
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	a, err := client.client.ReadTrc20Balance(string(address), string(client.contract))
	if err != nil {
		return xc.AmountBlockchain{}, err
	}

	return xc.NewAmountBlockchainFromStr(a.String()), nil
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	resp, err := client.client.GetAccount(string(address))
	if err != nil {
		return xc.AmountBlockchain{}, err
	}

	return xc.NewAmountBlockchainFromUint64(uint64(resp.Balance)), nil
}

func deserialiseTransactionEvents(log []*httpclient.Log) ([]*xc.LegacyTxInfoEndpoint, []*xc.LegacyTxInfoEndpoint) {
	sources := make([]*xc.LegacyTxInfoEndpoint, 0)
	destinations := make([]*xc.LegacyTxInfoEndpoint, 0)

	for _, event := range log {
		source := new(xc.LegacyTxInfoEndpoint)
		destination := new(xc.LegacyTxInfoEndpoint)
		source.NativeAsset = xc.TRX
		destination.NativeAsset = xc.TRX

		// The addresses in the TVM omits the prefix 0x41, so we add it here to allow us to parse the addresses
		eventContractB58 := base58.CheckEncode(event.Address, 0x41)
		eventSourceB58 := base58.CheckEncode(event.Topics[1][12:], 0x41)      // Remove padding
		eventDestinationB58 := base58.CheckEncode(event.Topics[2][12:], 0x41) // Remove padding
		eventMethodBz := event.Topics[0]

		eventValue := new(big.Int)
		eventValue.SetString(hex.EncodeToString(event.Data), 16) // event value is returned as a padded big int hex

		if hex.EncodeToString(eventMethodBz) != strings.TrimPrefix(TRANSFER_EVENT_HASH_HEX, "0x") {
			continue
		}

		source.ContractAddress = xc.ContractAddress(eventContractB58)
		destination.ContractAddress = xc.ContractAddress(eventContractB58)

		source.Address = xc.Address(eventSourceB58)
		source.Amount = xc.NewAmountBlockchainFromUint64(eventValue.Uint64())
		destination.Address = xc.Address(eventDestinationB58)
		destination.Amount = xc.NewAmountBlockchainFromUint64(eventValue.Uint64())

		sources = append(sources, source)
		destinations = append(destinations, destination)
	}

	return sources, destinations
}

func deserialiseNativeTransfer(tx *httpclient.GetTransactionIDResponse) (xc.Address, xc.Address, xc.AmountBlockchain, error) {
	if len(tx.RawData.Contract) != 1 {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction")
	}

	contract := tx.RawData.Contract[0]

	if contract.Type != "TransferContract" {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction")
	}
	transferContract, err := contract.AsTransferContract()
	if err != nil {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("invalid transfer-contract: %v", err)
	}

	from := xc.Address(transferContract.Owner)
	to := xc.Address(transferContract.To)
	amount := transferContract.Amount

	return from, to, xc.NewAmountBlockchainFromUint64(uint64(amount)), nil
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
	var tonBlock *httpclient.BlockResponse
	height, ok := args.Height()
	if !ok {
		response, err := client.client.GetBlockByLatest(1)
		if err != nil {
			return nil, err
		}
		if len(response.Block) == 0 {
			return nil, fmt.Errorf("no blocks found on chain")
		}
		tonBlock = response.Block[0]
	} else {
		response, err := client.client.GetBlockByNum(height)
		if err != nil {
			return nil, err
		}
		tonBlock = response
	}
	height = tonBlock.BlockHeader.RawData.Number

	txs := tonBlock.Transactions

	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(
			client.chain.Chain,
			height,
			tonBlock.BlockId,
			time.Unix(int64(tonBlock.BlockHeader.RawData.Timestamp/1000), 0),
		),
	}
	for _, tx := range txs {
		block.TransactionIds = append(block.TransactionIds, tx.TxID)
	}

	return block, nil
}
