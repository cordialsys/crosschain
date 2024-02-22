package tron

import (
	"context"
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/utils"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var _ xc.Client = &Client{}

const TRANSFER_EVENT_HASH_HEX = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

// Client for Template
type Client struct {
	client *client.GrpcClient

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

	client := client.NewGrpcClient(cfg.URL)
	err := client.Start(grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{client, xc.ContractAddress(cfgI.GetContract()), cfgI.GetChain().ExplorerURL}, nil
}

func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	input := new(TxInput)

	// Getting blockhash details from the CreateTransfer endpoint as TRON uses an unusual hashing algorithm (SHA2256SM3), so we can't do a minimal
	// retrieval and just get the blockheaders
	dummyTx, err := client.client.Transfer(string(from), string(to), 5)
	if err != nil {
		return nil, err
	}

	input.RefBlockBytes = dummyTx.Transaction.RawData.RefBlockBytes
	input.RefBlockHash = dummyTx.Transaction.RawData.RefBlockHash
	input.Expiration = dummyTx.Transaction.RawData.Expiration

	return input, nil
}

// SubmitTx submits a Tron tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	t := tx.(*Tx)

	_, err := client.client.Broadcast(t.tronTx)

	return err
}

func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xc.TxInfo, error) {
	tx, err := client.client.GetTransactionByID(string(txHash))
	if err != nil {
		return xc.TxInfo{}, err
	}

	info, err := client.client.GetTransactionInfoByID(string(txHash))
	if err != nil {
		return xc.TxInfo{}, err
	}

	block, err := client.client.GetBlockByNum(info.GetBlockNumber())
	if err != nil {
		return xc.TxInfo{}, err
	}

	var from xc.Address
	var to xc.Address
	var amount xc.AmountBlockchain
	sources, destinations := deserialiseTransactionEvents(info.GetLog())
	// If we cannot retrieve transaction events, we can infer that the TX is a native transfer
	if len(sources) == 0 && len(destinations) == 0 {
		from, to, amount, err = deserialiseNativeTransfer(tx)
		if err != nil {
			return xc.TxInfo{}, err
		}

		source := new(xc.TxInfoEndpoint)
		source.Address = from
		source.Amount = amount
		source.Asset = "TRX"
		source.NativeAsset = xc.TRX

		destination := new(xc.TxInfoEndpoint)
		destination.Address = to
		destination.Amount = amount
		destination.Asset = "TRX"
		destination.NativeAsset = xc.TRX

		sources = append(sources, source)
		destinations = append(destinations, destination)
	}

	txInfo := xc.TxInfo{
		BlockHash:       string(common.BytesToHexString(block.Blockid)),
		TxID:            string(txHash),
		ExplorerURL:     client.blockExplorerURL + fmt.Sprintf("/transaction/%s", string(txHash)),
		From:            from,
		To:              to,
		ContractAddress: xc.ContractAddress(address.HexToAddress(common.BytesToHexString(info.GetContractAddress())).String()),
		Amount:          amount,
		Fee:             xc.NewAmountBlockchainFromUint64(uint64(info.GetFee())),
		BlockIndex:      info.GetBlockNumber(),
		BlockTime:       info.GetBlockTimeStamp() / 1000,
		Confirmations:   0,
		Status:          xc.TxStatusSuccess,
		Sources:         sources,
		Destinations:    destinations,
		Time:            info.GetBlockTimeStamp(),
		TimeReceived:    0,
		Error:           "",
	}

	return txInfo, nil
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	a, err := client.client.TRC20ContractBalance(string(address), string(client.contract))
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

func deserialiseTransactionEvents(log []*core.TransactionInfo_Log) ([]*xc.TxInfoEndpoint, []*xc.TxInfoEndpoint) {
	sources := make([]*xc.TxInfoEndpoint, 0)
	destinations := make([]*xc.TxInfoEndpoint, 0)

	for _, event := range log {
		source := new(xc.TxInfoEndpoint)
		destination := new(xc.TxInfoEndpoint)
		source.NativeAsset = xc.TRX
		destination.NativeAsset = xc.TRX

		// The addresses in the TVM omits the prefix 0x41, so we add it here to allow us to parse the addresses
		eventContract := address.HexToAddress("0x41" + common.BytesToHexString(event.Address)[2:])
		eventMethod := common.BytesToHexString(event.Topics[0])
		eventSource := address.HexToAddress("0x41" + common.BytesToHexString(event.Topics[1])[26:])      // Remove padding
		eventDestination := address.HexToAddress("0x41" + common.BytesToHexString(event.Topics[2])[26:]) // Remove padding

		eventValue := new(big.Int)
		eventValue.SetString(common.BytesToHexString(event.Data), 0) // event value is returned as a padded big int hex

		if eventMethod != TRANSFER_EVENT_HASH_HEX {
			continue
		}

		source.ContractAddress = xc.ContractAddress(eventContract.String())
		destination.ContractAddress = xc.ContractAddress(eventContract.String())

		source.Address = xc.Address(eventSource.String())
		source.Amount = xc.NewAmountBlockchainFromUint64(eventValue.Uint64())
		destination.Address = xc.Address(eventDestination.String())
		destination.Amount = xc.NewAmountBlockchainFromUint64(eventValue.Uint64())

		sources = append(sources, source)
		destinations = append(destinations, destination)
	}

	return sources, destinations
}

func deserialiseNativeTransfer(tx *core.Transaction) (xc.Address, xc.Address, xc.AmountBlockchain, error) {
	if len(tx.RawData.Contract) != 1 {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction")
	}

	contract := tx.RawData.Contract[0]

	if contract.Type != core.Transaction_Contract_TransferContract {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction")
	}

	params := new(core.TransferContract)
	err := anypb.UnmarshalTo(contract.Parameter, params, proto.UnmarshalOptions{})
	if err != nil {
		return "", "", xc.AmountBlockchain{}, fmt.Errorf("unsupported transaction: %w", err)
	}

	from := xc.Address(address.Address(params.OwnerAddress).String())
	to := xc.Address(address.Address(params.ToAddress).String())
	amount := params.Amount

	return from, to, xc.NewAmountBlockchainFromUint64(uint64(amount)), nil
}
