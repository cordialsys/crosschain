package sui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coming-chat/go-sui/v2/client"
	"github.com/coming-chat/go-sui/v2/lib"
	"github.com/coming-chat/go-sui/v2/move_types"
	"github.com/coming-chat/go-sui/v2/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/sirupsen/logrus"
)

// Client for Sui
type Client struct {
	Asset     xc.ITask
	SuiClient *client.Client
}

// NewClient returns a new Sui Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	client, err := client.Dial(cfg.URL)
	return &Client{
		Asset:     cfgI,
		SuiClient: client,
	}, err
}

var _ xclient.FullClient = &Client{}

const GAS_BUDGET_PER_COIN = uint64(20_000_000)

type SuiMethod string

var (
	// getTransactionBlock SuiMethod = "sui_getTransactionBlock"
	getCheckpoint  SuiMethod = "sui_getCheckpoint"
	getCheckpoints SuiMethod = "sui_getCheckpoints"
	MaxCoinObjects int       = 50
)

func (m SuiMethod) String() string {
	return string(m)
}

type Checkpoint struct {
	Epoch                    string `json:"epoch"`
	SequenceNumber           string `json:"sequenceNumber"`
	Digest                   string `json:"digest"`
	NetworkTotalTransactions string `json:"networkTotalTransactions"`
	PreviousDigest           string `json:"PreviousDigest"`
	TimestampMs              string `json:"timestampMs"`
}

func (ch *Checkpoint) GetEpoch() uint64 {
	return xc.NewAmountBlockchainFromStr(ch.Epoch).Uint64()
}
func (ch *Checkpoint) GetSequenceNumber() uint64 {
	return xc.NewAmountBlockchainFromStr(ch.SequenceNumber).Uint64()
}

type Checkpoints struct {
	Data []*Checkpoint `json:"data"`
}

func (c *Client) FetchLatestCheckpoint(ctx context.Context) (*Checkpoint, error) {
	resp := &Checkpoints{}
	// get last 1 checkpoint, descending order
	err := c.SuiClient.CallContext(ctx, resp, getCheckpoints, nil, 1, true)
	if len(resp.Data) == 0 {
		return &Checkpoint{}, errors.New("no checkpoints yet")
	}
	return resp.Data[0], err
}

func (c *Client) FetchCheckpoint(ctx context.Context, checkpoint uint64) (*Checkpoint, error) {
	resp := &Checkpoint{}
	// get last 1 checkpoint, descending order
	err := c.SuiClient.CallContext(ctx, resp, getCheckpoint, fmt.Sprintf("%d", checkpoint))
	return resp, err
}

func AddressOrObjectOwner(obj *types.ObjectOwner) (string, bool) {
	if obj.AddressOwner != nil {
		return obj.AddressOwner.String(), true
	}
	if obj.ObjectOwner != nil {
		return obj.ObjectOwner.String(), true
	}
	if obj.SingleOwner != nil {
		return obj.SingleOwner.String(), true
	}
	return "", false
}

func (c *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	opts := types.SuiTransactionBlockResponseOptions{
		ShowInput:          true,
		ShowEffects:        true,
		ShowObjectChanges:  true,
		ShowBalanceChanges: true,
		// do we need events?
		ShowEvents: true,
	}
	txHashBz, err := lib.NewBase58(string(txHash))
	if err != nil || txHashBz == nil || len(txHashBz.Data()) < 10 || len(txHashBz.Data()) > 33 {
		return xc.LegacyTxInfo{}, errors.Join(errors.New("could not decode txHash"), err)
	}

	resp, err := c.SuiClient.GetTransactionBlock(ctx, *txHashBz, opts)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	// get latest checkpoint so we can compute our confirmations
	latestCheckpoint, err := c.FetchLatestCheckpoint(ctx)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	if resp.Checkpoint == nil {
		return xc.LegacyTxInfo{}, errors.New("sui endpoint failed to provide checkpoint")
	}
	txCheckpoint, err := c.FetchCheckpoint(ctx, resp.Checkpoint.Uint64())
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	// latestCheckpoint.Epoch
	sources := []*xc.LegacyTxInfoEndpoint{}
	destinations := []*xc.LegacyTxInfoEndpoint{}

	from := ""
	to := ""
	contract := ""
	destinationAmount := xc.NewAmountBlockchainFromUint64(0)
	totalSuiSent := xc.NewAmountBlockchainFromUint64(0)
	totalSuiReceived := xc.NewAmountBlockchainFromUint64(0)

	for _, bal := range resp.BalanceChanges {
		amt := xc.NewAmountBlockchainFromStr(bal.Amount)

		asset := ""
		contract = bal.CoinType
		isSui := false
		if strings.HasSuffix(bal.CoinType, "sui::SUI") && (strings.HasPrefix(bal.CoinType, "0x0000000000000000000000000000000000000000000000000000000000") || len(contract) < 16) {
			contract = ""
			asset = "SUI"
			isSui = true
		}
		balBz, _ := json.Marshal(bal)
		logrus.WithFields(logrus.Fields{
			"chain":     c.Asset.GetChain().Chain,
			"amount":    bal.Amount,
			"coin_type": bal.CoinType,
			"owner":     bal.Owner,
			"is_sui":    isSui,
			"bal":       string(balBz),
		}).Trace("balance change")
		if amt.Sign() < 0 {
			from, _ = AddressOrObjectOwner(&bal.Owner)
			abs := amt.Abs()
			if isSui {
				totalSuiSent = totalSuiSent.Add(&abs)
			}
			sources = append(sources, &xc.LegacyTxInfoEndpoint{
				Asset:           asset,
				ContractAddress: xc.ContractAddress(contract),
				Amount:          abs,
				Address:         xc.Address(from),
				NativeAsset:     xc.NativeAsset(c.Asset.GetChain().Chain),
			})
		} else {
			to, _ = AddressOrObjectOwner(&bal.Owner)
			destinationAmount = amt
			if isSui {
				totalSuiReceived = totalSuiReceived.Add(&amt)
			}
			destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
				Asset:           asset,
				ContractAddress: xc.ContractAddress(contract),
				Amount:          amt,
				Address:         xc.Address(to),
				NativeAsset:     xc.NativeAsset(c.Asset.GetChain().Chain),
			})
		}
	}

	// fee is difference between total sent and received in balance changes
	fee := totalSuiSent.Sub(&totalSuiReceived)
	logrus.WithFields(logrus.Fields{
		"total_sui_received": totalSuiReceived.String(),
		"total_sui_sent":     totalSuiSent.String(),
		"fee":                fee.String(),
	}).Trace("sui fee")

	status := xc.TxStatusSuccess
	if resp.Effects.Data.V1.Status.Error != "" {
		status = xc.TxStatusFailure
	}

	return xc.LegacyTxInfo{
		BlockHash:       txCheckpoint.Digest,
		TxID:            resp.Digest.String(),
		From:            xc.Address(from),
		To:              xc.Address(to),
		ContractAddress: xc.ContractAddress(contract),
		Amount:          destinationAmount,
		Fee:             fee,
		// should be in seconds
		BlockTime:     resp.TimestampMs.Int64() / 1000,
		BlockIndex:    resp.Checkpoint.Int64(),
		Confirmations: int64(latestCheckpoint.GetSequenceNumber()) - int64(txCheckpoint.GetSequenceNumber()),

		ExplorerURL:  fmt.Sprintf("https://explorer.sui.io/txblock/%s?network=%s", resp.Digest, c.Asset.GetChain().Net),
		Sources:      sources,
		Destinations: destinations,
		Error:        resp.Effects.Data.V1.Status.Error,
		Status:       status,
	}, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// delete the fee to avoid double counting.
	// Sui, like btc, counts fee as difference between total sent and recv, which is already automatically counted.
	legacyTx.Fee = xc.NewAmountBlockchainFromUint64(0)
	// remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain().Chain, legacyTx, xclient.Utxo), nil
}

func (c *Client) EstimateGas(ctx context.Context) (xc.AmountBlockchain, error) {
	ref, err := c.SuiClient.GetReferenceGasPrice(ctx)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}
	return xc.NewAmountBlockchainFromUint64(ref.Uint64()), nil
}

func (c *Client) GetAllCoinsFor(ctx context.Context, address xc.Address, contract string) ([]*types.Coin, error) {

	all_coins := []*types.Coin{}

	fromData, err := move_types.NewAccountAddressHex(string(address))
	if err != nil {
		return []*types.Coin{}, err
	}
	var next *move_types.AccountAddress
	for {
		coins, err := c.SuiClient.GetCoins(ctx, *fromData, &contract, next, 250)
		if err != nil {
			return []*types.Coin{}, err
		}
		for _, coin := range coins.Data {
			c := coin
			all_coins = append(all_coins, &c)
		}
		next = coins.NextCursor
		if next == nil || !coins.HasNextPage {
			break
		}
	}
	return all_coins, nil

}

func (c *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {

	// native asset SUI
	native := "0x2::sui::SUI"
	contract := native
	if token, ok := c.Asset.(*xc.TokenAssetConfig); ok {
		contract = NormalizeCoinContract(token.Contract)
	}

	all_coins, err := c.GetAllCoinsFor(ctx, args.GetFrom(), contract)
	if err != nil {
		return &TxInput{}, err
	}

	latestCheckpoint, err := c.FetchLatestCheckpoint(ctx)
	if err != nil {
		return &TxInput{}, err
	}
	epoch := xc.NewAmountBlockchainFromStr(latestCheckpoint.Epoch)

	// store the object id's for the transfer
	input := NewTxInput()
	input.CurrentEpoch = epoch.Uint64()
	input.Coins = all_coins
	input.SortCoins()
	// take max 50 to bound the tx_input size.
	if len(input.Coins) > MaxCoinObjects {
		input.Coins = input.Coins[:MaxCoinObjects]
	}

	if contract == native {
		// gas coin should just be the largest object
		if len(input.Coins) > 0 {
			input.GasCoin = *input.Coins[0]
		}
	} else {
		// we need to fetch our sui objects
		all_sui_coins, err := c.GetAllCoinsFor(ctx, args.GetFrom(), native)
		if err != nil {
			return &TxInput{}, err
		}
		SortCoins(all_sui_coins)
		if len(all_sui_coins) > 0 {
			input.GasCoin = *all_sui_coins[0]
		}
	}

	gasPrice, err := c.EstimateGas(ctx)
	if err != nil {
		defaultgas := c.Asset.GetChain().ChainGasPriceDefault
		if defaultgas < 0.1 {
			return input, err
		}
		// use the default
		input.GasPrice = uint64(defaultgas)
	}
	input.GasPrice = gasPrice.Uint64()
	// 2 SUI
	input.GasBudget = 2_000_000_000

	// Incrementally increase budget per additional coin being consumed
	input.GasBudget = input.GasBudget + GAS_BUDGET_PER_COIN*uint64(len(input.Coins))

	input.ExcludeGasCoin()

	return input, nil
}
func (c *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return c.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Sui tx
func (c *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	tx_bz, err := tx.Serialize()
	if err != nil {
		return err
	}
	// var sigs [][]byte
	sigsB64 := []any{}
	sigs := tx.GetSignatures()

	for _, sig := range sigs {
		sigsB64 = append(sigsB64, lib.Base64Data(sig))
	}

	newTxn, err := c.SuiClient.ExecuteTransactionBlock(
		ctx,
		lib.Base64Data(tx_bz),
		sigsB64,
		&types.SuiTransactionBlockResponseOptions{},
		types.TxnRequestTypeWaitForLocalExecution,
	)
	_ = newTxn
	return err
}

func (c *Client) FetchBalanceFor(ctx context.Context, address xc.Address, contract string) (xc.AmountBlockchain, error) {
	total := xc.NewAmountBlockchainFromUint64(0)
	contract = NormalizeCoinContract(contract)
	all_coins, err := c.GetAllCoinsFor(ctx, address, contract)
	if err != nil {
		return total, err
	}

	for _, coin := range all_coins {
		amt := xc.NewAmountBlockchainFromUint64(coin.Balance.Uint64())
		total = total.Add(&amt)
	}

	return total, nil
}
func (c *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	// native asset SUI
	contract := "0x2::sui::SUI"
	if token, ok := c.Asset.(*xc.TokenAssetConfig); ok {
		contract = token.Contract
	}
	return c.FetchBalanceFor(ctx, address, contract)
}

func (c *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return c.FetchBalanceFor(ctx, address, "0x2::sui::SUI")
}
