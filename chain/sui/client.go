package sui

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/coming-chat/go-sui/v2/client"
	"github.com/coming-chat/go-sui/v2/lib"
	"github.com/coming-chat/go-sui/v2/move_types"
	"github.com/coming-chat/go-sui/v2/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
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

var _ xclient.Client = &Client{}

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
	Epoch                    string   `json:"epoch"`
	SequenceNumber           string   `json:"sequenceNumber"`
	Digest                   string   `json:"digest"`
	NetworkTotalTransactions string   `json:"networkTotalTransactions"`
	PreviousDigest           string   `json:"PreviousDigest"`
	TimestampMs              string   `json:"timestampMs"`
	Transactions             []string `json:"transactions"`
	CheckpointCommitments    []string `json:"checkpointCommitments"`
	ValidatorSignature       string   `json:"validatorSignature"`
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
		return &Checkpoint{}, fmt.Errorf("no checkpoints yet")
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

func isMissingTransactionErr(err error) bool {
	if err == nil {
		return false
	}
	// SUI does not return a specific error code in JSON RPC for missing transaction,
	// so we must string match.
	if strings.Contains(strings.ToLower(err.Error()), "could not find the referenced transaction") {
		return true
	}

	return false
}

func (c *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	opts := types.SuiTransactionBlockResponseOptions{
		ShowInput:          true,
		ShowEffects:        true,
		ShowObjectChanges:  true,
		ShowBalanceChanges: true,
		// do we need events?
		ShowEvents: true,
	}
	txHashBz, err := lib.NewBase58(string(txHash))
	if err != nil || txHashBz == nil || len(*txHashBz) == 0 {
		return xclient.LegacyTxInfo{}, fmt.Errorf("could not decode txHash: %v", err)
	}

	resp, err := c.SuiClient.GetTransactionBlock(ctx, *txHashBz, opts)
	if err != nil {
		if isMissingTransactionErr(err) {
			return xclient.LegacyTxInfo{}, errors.TransactionNotFoundf("%v", err)
		}
		return xclient.LegacyTxInfo{}, fmt.Errorf("could not get transaction block: %v", err)
	}

	// get latest checkpoint so we can compute our confirmations
	latestCheckpoint, err := c.FetchLatestCheckpoint(ctx)
	if err != nil {
		return xclient.LegacyTxInfo{}, fmt.Errorf("could not get latest checkpoint: %v", err)
	}
	if resp.Checkpoint == nil {
		return xclient.LegacyTxInfo{}, fmt.Errorf("sui endpoint failed to provide checkpoint")
	}
	txCheckpoint, err := c.FetchCheckpoint(ctx, resp.Checkpoint.Uint64())
	if err != nil {
		return xclient.LegacyTxInfo{}, fmt.Errorf("could not get checkpoint %d: %v", resp.Checkpoint.Uint64(), err)
	}
	// latestCheckpoint.Epoch
	sources := []*xclient.LegacyTxInfoEndpoint{}
	destinations := []*xclient.LegacyTxInfoEndpoint{}

	from := ""
	to := ""
	contract := ""
	destinationAmount := xc.NewAmountBlockchainFromUint64(0)
	totalSuiSent := xc.NewAmountBlockchainFromUint64(0)
	totalSuiReceived := xc.NewAmountBlockchainFromUint64(0)

	for i, bal := range resp.BalanceChanges {
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
			sources = append(sources, &xclient.LegacyTxInfoEndpoint{
				Asset:           asset,
				ContractAddress: xc.ContractAddress(contract),
				Amount:          abs,
				Address:         xc.Address(from),
				NativeAsset:     xc.NativeAsset(c.Asset.GetChain().Chain),
				Event:           xclient.NewEventFromIndex(uint64(i), xclient.MovementVariantNative),
			})
		} else {
			to, _ = AddressOrObjectOwner(&bal.Owner)
			destinationAmount = amt
			if isSui {
				totalSuiReceived = totalSuiReceived.Add(&amt)
			}
			destinations = append(destinations, &xclient.LegacyTxInfoEndpoint{
				Asset:           asset,
				ContractAddress: xc.ContractAddress(contract),
				Amount:          amt,
				Address:         xc.Address(to),
				NativeAsset:     xc.NativeAsset(c.Asset.GetChain().Chain),
				Event:           xclient.NewEventFromIndex(uint64(i), xclient.MovementVariantNative),
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

	return xclient.LegacyTxInfo{
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
	return xclient.TxInfoFromLegacy(client.Asset.GetChain(), legacyTx, xclient.Utxo), nil
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
	native := c.Asset.GetChain().ChainCoin
	if native == "" {
		native = "0x2::sui::SUI"
	}
	contract := native
	if contractInput, ok := args.GetContract(); ok {
		contract = string(contractInput)
	}
	contract = NormalizeCoinContract(contract)

	input := NewTxInput()
	suiCoins, err := c.GetAllCoinsFor(ctx, args.GetFrom(), native)
	if err != nil {
		return &TxInput{}, err
	}
	SortCoins(suiCoins)
	if len(suiCoins) > 0 {
		// use the largest SUI coin for gas
		input.GasCoin = *suiCoins[0]
		input.GasCoinOwner = args.GetFrom()
	}
	feePayer, ok := args.GetFeePayer()
	if ok {
		input.GasCoin = types.Coin{}
		sponsorSuiCoins, err := c.GetAllCoinsFor(ctx, feePayer, native)
		if err != nil {
			return &TxInput{}, err
		}
		// use gas coin from sponsor if available
		SortCoins(sponsorSuiCoins)
		if len(sponsorSuiCoins) > 0 {
			input.GasCoin = *sponsorSuiCoins[0]
			input.GasCoinOwner = feePayer
		} else {
			return &TxInput{}, fmt.Errorf("SUI fee payer %s has no SUI coins", feePayer)
		}
	}

	transferCoins := suiCoins
	if contract != native && contract != "" {
		// If we're not sending SUI, we need to make a separate call to get coins that are being transferred.
		transferCoins, err = c.GetAllCoinsFor(ctx, args.GetFrom(), contract)
		if err != nil {
			return &TxInput{}, err
		}
		SortCoins(transferCoins)
	}

	latestCheckpoint, err := c.FetchLatestCheckpoint(ctx)
	if err != nil {
		return &TxInput{}, err
	}
	epoch := xc.NewAmountBlockchainFromStr(latestCheckpoint.Epoch)
	input.CurrentEpoch = epoch.Uint64()

	// store the object id's for the transfer
	input.Coins = transferCoins
	input.SortCoins()
	// take max 50 to bound the tx_input size.
	if len(input.Coins) > MaxCoinObjects {
		input.Coins = input.Coins[:MaxCoinObjects]
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
	inputSim := *input
	if _, ok := args.GetPublicKey(); !ok {
		args.SetPublicKey(make([]byte, 32))
	}

	builder, err := NewTxBuilder(c.Asset.GetChain().Base())
	if err != nil {
		return input, fmt.Errorf("could not create tx builder: %v", err)
	}

	txI, err := builder.Transfer(args, &inputSim)
	if err != nil {
		return input, fmt.Errorf("could not build tx: %v", err)
	}
	tx := txI.(*Tx)
	serialized, err := tx.Serialize()
	if err != nil {
		return input, fmt.Errorf("could not serialize tx: %v", err)
	}
	dryRun, err := c.SuiClient.DryRunTransaction(ctx, lib.Base64Data(serialized))
	if err != nil {
		return input, fmt.Errorf("could not dry run tx: %v", err)
	}
	log := logrus.WithField("from", args.GetFrom())
	if dryRun.Effects.Data.V1 == nil {
		log.Error("dry run returned nil effects")
	} else {
		// outBz, _ := json.MarshalIndent(dryRun, "", "  ")
		// fmt.Println(string(outBz))
		log = log.WithField("status", dryRun.Effects.Data.V1.Status.Status)
		log = log.WithField("error", dryRun.Effects.Data.V1.Status.Error)
		if dryRun.Effects.Data.V1.Status.Status == "success" {
			gasUsed := dryRun.Effects.Data.V1.GasUsed
			// https://docs.sui.io/concepts/tokenomics/gas-in-sui
			gasFee := gasUsed.ComputationCost.Uint64() + gasUsed.StorageCost.Uint64()
			gasRebate := gasUsed.StorageRebate.Uint64()
			// use the min gas budget for SUI
			if gasRebate > gasFee {
				gasFee = c.Asset.GetChain().GasBudgetMinimum.ToBlockchain(c.Asset.GetChain().Decimals).Uint64()
				if gasFee == 0 {
					gasFee = 2000000
				}
			} else {
				gasFee = gasFee - gasRebate
			}

			if contract != native {
				// increase budget by 10% for 3rd party coins
				log = log.WithField("contract", contract)
				gasFee = (gasFee * 110) / 100
			}
			input.GasBudget = gasFee
			log = log.WithField("gas_budget", gasFee)
		}
		log.Debug("simulated tx")
	}

	return input, nil
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
func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	// native asset SUI should be something like "0x2::sui::SUI"
	contractToUse := c.Asset.GetChain().ChainCoin
	if contractToUse == "" {
		contractToUse = "0x2::sui::SUI"
	}

	if contract, ok := args.Contract(); ok {
		contractToUse = string(contract)
	}

	return c.FetchBalanceFor(ctx, args.Address(), contractToUse)
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	meta, err := client.SuiClient.GetCoinMetadata(ctx, string(contract))
	if err != nil {
		return 0, err
	}
	return int(meta.Decimals), nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, ok := args.Height()
	if !ok {
		seq, err := client.SuiClient.GetLatestCheckpointSequenceNumber(ctx)
		if err != nil {
			return nil, err
		}
		asInt := big.NewInt(0)
		_, ok := asInt.SetString(seq, 0)
		if !ok {
			return nil, fmt.Errorf("received invalid sequence: %s", seq)
		}
		height = asInt.Uint64()
	}

	checkpoint := &Checkpoint{}
	err := client.SuiClient.CallContext(ctx, checkpoint, getCheckpoint, fmt.Sprint(height))
	if err != nil {
		return nil, err
	}

	timestampMs, _ := strconv.ParseUint(checkpoint.TimestampMs, 10, 64)
	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(
			client.Asset.GetChain().Chain,
			height,
			checkpoint.Digest,
			time.Unix(int64(timestampMs/1000), 0),
		),
	}
	block.TransactionIds = append(block.TransactionIds, checkpoint.Transactions...)
	return block, nil
}
