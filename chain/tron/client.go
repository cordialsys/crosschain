package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	httpclient "github.com/cordialsys/crosschain/chain/tron/http_client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/utils"
	"github.com/okx/go-wallet-sdk/crypto/base58"
)

var _ xclient.Client = &Client{}

const TRANSFER_EVENT_HASH_HEX = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

// Client for Template
type Client struct {
	// client *client.GrpcClient
	client *httpclient.Client

	contract         xc.ContractAddress
	blockExplorerURL string
}

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	utils.TxPriceInput

	// 6th to 8th (exclusive) byte of the reference block height
	RefBlockBytes []byte
	// 8th to 16th (exclusive) byte of the reference block hash
	RefBlockHash []byte

	Expiration int64
	Timestamp  int64
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverTron,
		},
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

	return &Client{client, xc.ContractAddress(cfgI.GetContract()), cfgI.GetChain().ExplorerURL}, nil
}

func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	input := new(TxInput)

	// Getting blockhash details from the CreateTransfer endpoint as TRON uses an unusual hashing algorithm (SHA2256SM3), so we can't do a minimal
	// retrieval and just get the blockheaders
	dummyTx, err := client.client.CreateTransaction(string(from), string(to), 5)
	if err != nil {
		return nil, err
	}

	input.RefBlockBytes = dummyTx.RawData.RefBlockBytes
	input.RefBlockHash = dummyTx.RawData.RefBlockHashBytes
	// give 2 hours (miliseconds)
	input.Expiration = time.Now().Add(time.Hour * 2).UnixMilli()

	return input, nil
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
			return xc.LegacyTxInfo{}, err
		}

		source := new(xc.LegacyTxInfoEndpoint)
		source.Address = from
		source.Amount = amount
		source.Asset = "TRX"
		source.NativeAsset = xc.TRX

		destination := new(xc.LegacyTxInfoEndpoint)
		destination.Address = to
		destination.Amount = amount
		destination.Asset = "TRX"
		destination.NativeAsset = xc.TRX

		sources = append(sources, source)
		destinations = append(destinations, destination)
	}

	txInfo := xc.LegacyTxInfo{
		BlockHash:       block.BlockId,
		TxID:            string(txHash),
		ExplorerURL:     client.blockExplorerURL + fmt.Sprintf("/transaction/%s", string(txHash)),
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

	return txInfo, nil
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
