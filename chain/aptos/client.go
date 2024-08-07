package aptos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coming-chat/go-aptos/aptosclient"
	"github.com/coming-chat/go-aptos/aptostypes"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
)

// Client for Aptos
type Client struct {
	Asset       xc.ITask
	AptosClient *aptosclient.RestClient
}

var _ xclient.FullClient = &Client{}

// NewClient returns a new Aptos Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	client, err := aptosclient.Dial(context.Background(), cfg.URL)
	return &Client{
		Asset:       cfgI,
		AptosClient: client,
	}, err
}

// Enable constructing multiple clients without dialing aptos endpoint
// multiple times
func NewClientFrom(asset xc.ITask, client *aptosclient.RestClient) *Client {
	return &Client{
		Asset:       asset,
		AptosClient: client,
	}
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	ledger, err := client.AptosClient.LedgerInfo()
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	acc, err := client.AptosClient.GetAccount(string(args.GetFrom()))
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	gas_price, err := client.EstimateGas(ctx, ledger)
	if err != nil {
		return &tx_input.TxInput{}, err
	}

	return &tx_input.TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverAptos,
		},
		SequenceNumber: acc.SequenceNumber,
		ChainId:        ledger.ChainId,
		GasLimit:       2000,
		Timestamp:      ledger.LedgerTimestamp,
		GasPrice:       gas_price.Uint64(),
	}, nil
}

// FetchLegacyTxInput returns tx input for a Aptos tx
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Aptos tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	tx_bz, err := tx.Serialize()
	if err != nil {
		return err
	}
	newTxn, err := client.AptosClient.SubmitSignedBCSTransaction(tx_bz)
	_ = newTxn
	return err
}

type ChangeAndEvents struct {
	Change AptosChangeInner
	Events []aptostypes.Event
}

func (ch *ChangeAndEvents) ContractAddress() string {
	return parseContractAddress(ch.Change.Type)
}

type AptosChangeInner struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
type CoinStoreChange struct {
	DepositEvents  AptosEvents `json:"deposit_events"`
	WithdrawEvents AptosEvents `json:"withdraw_events"`
}
type AptosEvents struct {
	Counter string `json:"counter"`
	Guid    GuidId `json:"guid"`
}
type EventId struct {
	CreationNumber string `json:"creation_num"`
	AccountAddress string `json:"addr"`
}
type GuidId struct {
	Id EventId `json:"id"`
}

type CoinDepositEvent struct {
	Amount string `json:"amount"`
}
type CoinWithdrawEvent = CoinDepositEvent // same structure

func reserializeJson(obj any, target any) error {
	bz, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(bz, target)
}

// Transform "0x1::coin::CoinStore<x>" -> x
func parseContractAddress(typeString string) string {
	typeString = strings.Replace(typeString, "0x1::coin::CoinStore<", "", 1)
	typeString = strings.Replace(typeString, ">", "", 1)
	return typeString
}

// FetchLegacyTxInfo returns tx info for a Aptos tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {

	tx, err := client.AptosClient.GetTransactionByHash(string(txHash))
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	block, err := client.AptosClient.GetBlockByVersion(fmt.Sprintf("%d", tx.Version), false)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	ledger, err := client.AptosClient.LedgerInfo()
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	tx_height := block.BlockHeight
	now_height := ledger.BlockHeight
	confirmations := now_height - tx_height

	unit_price := tx.GasUnitPrice
	gas_used := tx.GasUsed
	feeu256 := xc.NewAmountBlockchainFromUint64(gas_used * unit_price)

	coinChanges := []ChangeAndEvents{}
	// we look at the changes in a transaction and join them with the events.
	// From this view, we can see which coins moved where.
	for _, ch := range tx.Changes {
		changeInner := AptosChangeInner{}
		err := reserializeJson(ch.Data, &changeInner)
		if err != nil {
			return xc.LegacyTxInfo{}, fmt.Errorf("could not deserialize aptos change")
		}
		if strings.HasPrefix(changeInner.Type, "0x1::coin::CoinStore") {
			change := &CoinStoreChange{}
			err := json.Unmarshal(changeInner.Data, change)
			if err != nil {
				return xc.LegacyTxInfo{}, fmt.Errorf("could not deserialize aptos change")
			}
			changeAndEvents := ChangeAndEvents{
				Change: changeInner,
			}

			for _, ev := range tx.Events {
				if ev.Guid.AccountAddress == change.DepositEvents.Guid.Id.AccountAddress &&
					ev.Guid.CreationNumber == change.DepositEvents.Guid.Id.CreationNumber {
					changeAndEvents.Events = append(changeAndEvents.Events, ev)
				} else if ev.Guid.AccountAddress == change.WithdrawEvents.Guid.Id.AccountAddress &&
					ev.Guid.CreationNumber == change.WithdrawEvents.Guid.Id.CreationNumber {
					changeAndEvents.Events = append(changeAndEvents.Events, ev)
				}
			}
			coinChanges = append(coinChanges, changeAndEvents)
		}
	}
	sources := []*xc.LegacyTxInfoEndpoint{}
	destinations := []*xc.LegacyTxInfoEndpoint{}

	for _, coinChange := range coinChanges {
		for _, ev := range coinChange.Events {
			switch ev.Type {
			case "0x1::coin::WithdrawEvent":
				withdraw := &CoinWithdrawEvent{}
				err := reserializeJson(ev.Data, withdraw)
				if err != nil {
					logrus.WithField("txhash", txHash).WithError(err).Error("could not deserialize aptos coin event")
					continue
				}
				contract := xc.ContractAddress(coinChange.ContractAddress())
				logrus.WithFields(logrus.Fields{
					"chain":    client.Asset.GetChain().Chain,
					"contract": contract,
					"address":  xc.Address(ev.Guid.AccountAddress),
					"amount":   withdraw.Amount,
				}).Debug("withdraw-event")
				sources = append(sources, &xc.LegacyTxInfoEndpoint{
					ContractAddress:            contract,
					LegacyAptosContractAddress: string(contract),
					NativeAsset:                client.Asset.GetChain().Chain,
					Address:                    xc.Address(ev.Guid.AccountAddress),
					Amount:                     xc.NewAmountBlockchainFromStr(withdraw.Amount),
				})
			case "0x1::coin::DepositEvent":
				deposit := &CoinDepositEvent{}
				err := reserializeJson(ev.Data, deposit)
				if err != nil {
					logrus.WithField("txhash", txHash).WithError(err).Error("could not deserialize aptos coin event")
					continue
				}
				contract := xc.ContractAddress(coinChange.ContractAddress())
				logrus.WithFields(logrus.Fields{
					"chain":    client.Asset.GetChain().Chain,
					"contract": contract,
					"address":  xc.Address(ev.Guid.AccountAddress),
					"amount":   deposit.Amount,
				}).Debug("deposit-event")
				destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
					ContractAddress:            contract,
					LegacyAptosContractAddress: string(contract),
					NativeAsset:                client.Asset.GetChain().Chain,
					Address:                    xc.Address(ev.Guid.AccountAddress),
					Amount:                     xc.NewAmountBlockchainFromStr(deposit.Amount),
				})
			default:
				// skip / unknown.
				logrus.WithFields(logrus.Fields{
					"event": ev.Type,
				}).Debug("unknown event")
			}
		}
	}

	// Legacy behavior expects that ContractAddress is blank for Aptos native asset -- this is not done
	// for new txinfo endpoint.
	for _, endpoint := range sources {
		if endpoint.ContractAddress == "0x1::aptos_coin::AptosCoin" {
			endpoint.ContractAddress = ""
		}
	}
	for _, endpoint := range destinations {
		if endpoint.ContractAddress == "0x1::aptos_coin::AptosCoin" {
			endpoint.ContractAddress = ""
		}
	}

	// destinations := destinationsFromTxPayload(tx.Payload)
	to := xc.Address("")
	amount := xc.NewAmountBlockchainFromUint64(0)
	if len(destinations) > 0 {
		to = destinations[0].Address
		amount = destinations[0].Amount
	}

	return xc.LegacyTxInfo{
		To:            to,
		From:          xc.Address(tx.Sender),
		Amount:        amount,
		Sources:       sources,
		Destinations:  destinations,
		Fee:           feeu256,
		Confirmations: int64(confirmations),
		BlockHash:     fmt.Sprintf("%d", tx.Version),
		// convert usec to sec
		BlockTime:   int64((tx.Timestamp / 1000) / 1000),
		TxID:        tx.Hash,
		BlockIndex:  int64(tx.Version),
		ExplorerURL: fmt.Sprintf("/txn/%d?network=%s", tx.Version, client.Asset.GetChain().Net),
	}, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain().Chain
	// undo the legacy behavior
	for _, endpoint := range legacyTx.Sources {
		endpoint.ContractAddress = xc.ContractAddress(endpoint.LegacyAptosContractAddress)
	}
	for _, endpoint := range legacyTx.Destinations {
		endpoint.ContractAddress = xc.ContractAddress(endpoint.LegacyAptosContractAddress)
	}

	// manually set the fee to avoid using `chains/APTOS/assets/APTOS` as we should use the correct contract for aptos.
	if legacyTx.FeeContract == "" {
		legacyTx.FeeContract = "0x1::aptos_coin::AptosCoin"
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Utxo), nil
}

// FetchBalance fetches balance for an Aptos address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	if token, ok := client.Asset.(*xc.TokenAssetConfig); ok {
		balance, err := client.AptosClient.BalanceOf(string(address), token.Contract)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), err
		}
		return xc.AmountBlockchain(*balance), err
	}
	return client.FetchNativeBalance(ctx, address)
}

// FetchNativeBalance fetches the native asset balance for an Aptos address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	balance, err := client.AptosClient.AptosBalanceOf(string(address))
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}
	return xc.AmountBlockchain(*balance), nil
}

func (client *Client) EstimateGas(ctx context.Context, ledgerInfo *aptostypes.LedgerInfo) (xc.AmountBlockchain, error) {

	// estimate using last 1 blocks
	zero := xc.NewAmountBlockchainFromUint64(0)
	height := ledgerInfo.BlockHeight
	if height < 500 {
		return zero, errors.New("the chain is too young")
	}
	attempts := 10

	// let's download the last 50 transactions
	transactions := []aptostypes.Transaction{}
	for len(transactions) < 50 && height > 0 {
		block, err := client.AptosClient.GetBlockByHeight(fmt.Sprintf("%d", height), true)
		height = height - 1
		if err != nil {
			// Sometimes a block doesn't exist..
			// so we'll tolerate up to 10 times of this in a row.
			attempts = attempts - 1
			if attempts <= 0 {
				return zero, err
			}
			continue
		}
		l1 := len(transactions)
		for _, tx := range block.Transactions {
			if tx.GasUnitPrice != 0 {
				transactions = append(transactions, tx)
			}
		}
		l2 := len(transactions)
		if l1 == l2 {
			// if the block was empty, count as a failed attempt so we will terminate
			attempts = attempts - 1
			if attempts <= 0 {
				break
			}
			continue
		}
		attempts = 10
	}
	totalUnitPrice := uint64(0)
	for _, tx := range transactions {
		totalUnitPrice += tx.GasUnitPrice
	}

	// use default of 0.000001 fee per gas
	if totalUnitPrice == 0 {
		return xc.NewAmountBlockchainFromUint64(100), nil
	}

	averUnitPrice := float32(totalUnitPrice) / float32(len(transactions))
	// pay 25% premium
	// averUnitPrice = averUnitPrice * 1.25
	// truncate
	unit_price := xc.NewAmountBlockchainFromUint64(uint64(averUnitPrice))

	return unit_price, nil
}
