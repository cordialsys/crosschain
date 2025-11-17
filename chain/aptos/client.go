package aptos

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/coming-chat/go-aptos/aptosclient"
	"github.com/coming-chat/go-aptos/aptostypes"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/aptos/events"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/sirupsen/logrus"
)

// Client for Aptos
type Client struct {
	Asset       xc.ITask
	AptosClient *aptosclient.RestClient
}

var _ xclient.Client = &Client{}

const DefaultGasLimit = 10000

// NewClient returns a new Aptos Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	client, err := aptosclient.DialWithClient(context.Background(), cfg.URL, httpClient)
	return &Client{
		cfgI,
		client,
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

func reserialize(data interface{}, into interface{}) error {
	bz, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bz, into)
}

type GasScheduleEntry struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

type GasSchedule struct {
	Entries []GasScheduleEntry `json:"entries"`
}

func (gasSchedule *GasSchedule) Get(key string) (uint64, error) {
	for _, entry := range gasSchedule.Entries {
		if entry.Key == key {
			val, err := strconv.ParseUint(entry.Val, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("could not parse %s: %s: %v", key, entry.Val, err)
			}
			return val, nil
		}
	}
	return 0, fmt.Errorf("could not get %s", key)
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
	gasScheduleResource, err := client.AptosClient.GetAccountResource("0x1", "0x1::gas_schedule::GasScheduleV2", 0)
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	gas_price, err := client.EstimateGas(ctx, ledger)
	if err != nil {
		return &tx_input.TxInput{}, err
	}

	gasSchedule := GasSchedule{}
	err = reserialize(gasScheduleResource.Data, &gasSchedule)
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	scheduleMinGasUnits, err := gasSchedule.Get("txn.min_transaction_gas_units")
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	scheduleMinGasPrice, err := gasSchedule.Get("txn.min_price_per_gas_unit")
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	scheduleScalingFactor, err := gasSchedule.Get("txn.gas_unit_scaling_factor")
	if err != nil {
		return &tx_input.TxInput{}, err
	}
	calculatedMinGasUnits := uint64(math.Ceil(float64(scheduleMinGasUnits) / float64(scheduleScalingFactor)))

	builder, err := NewTxBuilder(client.Asset.GetChain().Base())
	if err != nil {
		return &tx_input.TxInput{}, fmt.Errorf("could not create tx builder: %v", err)
	}
	defaultGasLimit := DefaultGasLimit
	if client.Asset.GetChain().GasLimitDefault > 0 {
		defaultGasLimit = client.Asset.GetChain().GasLimitDefault
	}
	input := &tx_input.TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverAptos,
		},
		SequenceNumber: acc.SequenceNumber,
		ChainId:        ledger.ChainId,
		GasLimit:       uint64(defaultGasLimit),
		Timestamp:      ledger.LedgerTimestamp,
		GasPrice:       gas_price.Uint64(),
	}

	// If the public key is set, we can simulate the tx and get
	// an accurate gas limit.
	if pubkey, ok := args.GetPublicKey(); ok {
		txI, err := builder.Transfer(args, input)
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not create tx: %v", err)
		}
		tx := txI.(*Tx)

		hashes, err := tx.Sighashes()
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not get sighashes: %v", err)
		}
		// Create a (fake) signature for each sign-request so we can simulate the tx gas with accuracy
		signatures := []*xc.SignatureResponse{}
		for i, hash := range hashes {
			zero := [32]byte{}
			zero[0] = byte(i)
			privateKey := ed25519.NewKeyFromSeed(zero[:])
			signatureData := ed25519.Sign(privateKey, hashes[0].Payload)
			address := args.GetFrom()
			publicKeyForSigner := pubkey
			if hash.Signer != "" && hash.Signer != address {
				publicKeyForSigner, _ = args.GetFeePayerPublicKey()
				address = hash.Signer
			}
			signatures = append(signatures, &xc.SignatureResponse{
				Signature: signatureData,
				PublicKey: publicKeyForSigner,
				Address:   address,
			})
		}
		err = tx.SetSignatures(signatures...)
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not set signatures: %v", err)
		}

		serialized, err := tx.Serialize()
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not serialize tx: %v", err)
		}

		output, err := client.AptosClient.SimulateSignedBCSTransaction(serialized)
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not simulate tx: %v", err)
		}
		log := logrus.WithFields(logrus.Fields{
			"gas_limit":  input.GasLimit,
			"public_key": hex.EncodeToString(pubkey),
			"from":       args.GetFrom(),
		})
		var success bool
		if len(output) > 0 {
			success = output[0].Success
			log = log.WithField("status", output[0].VmStatus).WithField("gas_used", output[0].GasUsed)
			if success {
				input.GasLimit = output[0].GasUsed
				// increase limit by ~10% for tokens it can vary sometimes.
				if _, ok := args.GetContract(); ok {
					input.GasLimit = (input.GasLimit * 1100) / 1000
				}
			}
		}

		log.WithField("success", success).Debug("simulated tx")
	} else {
		logrus.WithFields(logrus.Fields{
			"from": args.GetFrom(),
		}).Debug("cannot simulate tx, public key is not known")
	}
	estimatedGasPrice := input.GasPrice
	estimatedGasLimit := input.GasLimit
	if input.GasLimit < calculatedMinGasUnits {
		input.GasLimit = scheduleMinGasUnits
	}
	if input.GasPrice < scheduleMinGasPrice {
		input.GasPrice = scheduleMinGasPrice
	}
	if input.SequenceNumber == 0 {
		// The estimated gas sometimes is too low for the first txn, so we add a buffer.
		// Experimentally I found it was short by ~20 units, but couldn't find a good way to account for it,
		// so a generous buffer is added.
		input.GasLimit += 250
	}
	logrus.WithFields(logrus.Fields{
		"original_gas_limit":            estimatedGasLimit,
		"gas_limit":                     input.GasLimit,
		"original_gas_price":            estimatedGasPrice,
		"gas_price":                     input.GasPrice,
		"calculated_min_gas_units":      calculatedMinGasUnits,
		"txn.min_transaction_gas_units": scheduleMinGasUnits,
		"txn.min_price_per_gas_unit":    scheduleMinGasPrice,
		"txn.gas_unit_scaling_factor":   scheduleScalingFactor,
		"multiplier":                    client.Asset.GetChain().ChainGasMultiplier,
	}).Debug("gas limit")

	gasMultiplier := client.Asset.GetChain().ChainGasMultiplier
	if gasMultiplier > 0.01 {
		input.GasLimit = uint64(float64(input.GasLimit) * gasMultiplier)
	}

	return input, nil
}

// FetchLegacyTxInput returns tx input for a Aptos tx
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Aptos tx
func (client *Client) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq) error {
	tx_bz, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("could not serialize tx: %v", err)
	}
	newTxn, err := client.AptosClient.SubmitSignedBCSTransaction(tx_bz)
	_ = newTxn
	return err
}

// FetchLegacyTxInfo returns tx info for a Aptos tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	tx, err := client.AptosClient.GetTransactionByHash(string(txHash))
	if err != nil {
		if aptosErr, ok := err.(*aptostypes.RestError); ok {
			if aptosErr.Code == http.StatusNotFound {
				return txinfo.LegacyTxInfo{}, errors.TransactionNotFoundf("%v", err)
			}
		}
		return txinfo.LegacyTxInfo{}, err
	}

	block, err := client.AptosClient.GetBlockByVersion(fmt.Sprintf("%d", tx.Version), false)
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}
	ledger, err := client.AptosClient.LedgerInfo()
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}

	tx_height := block.BlockHeight
	now_height := ledger.BlockHeight
	confirmations := now_height - tx_height

	unit_price := tx.GasUnitPrice
	gas_used := tx.GasUsed
	feeu256 := xc.NewAmountBlockchainFromUint64(gas_used * unit_price)
	feePayerAddress := ""
	if tx.Signature != nil {
		if tx.Signature.FeePayerAddress != "" {
			feePayerAddress = tx.Signature.FeePayerAddress
		}
	}

	sources, destinations, err := events.ParseEvents(tx, txHash)
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}

	chainCfg := client.Asset.GetChain()
	// APTOS is inconsistent with how they report the native asset.
	// It can be either:
	// - 0x1::aptos_coin::AptosCoin
	// - 0xa
	// Also, we need to report it as "APTOS" to be consistent with other chains.
	for _, endpoint := range sources {
		nativeAsset, ok := chainCfg.FindAdditionalNativeAsset(endpoint.ContractAddress)
		if ok {
			endpoint.ContractId = endpoint.ContractAddress
			endpoint.ContractAddress = xc.ContractAddress(nativeAsset.AssetId)
		}
	}
	for _, endpoint := range destinations {
		nativeAsset, ok := chainCfg.FindAdditionalNativeAsset(endpoint.ContractAddress)
		if ok {
			endpoint.ContractId = endpoint.ContractAddress
			endpoint.ContractAddress = xc.ContractAddress(nativeAsset.AssetId)
		}
	}

	// destinations := destinationsFromTxPayload(tx.Payload)
	to := xc.Address("")
	amount := xc.NewAmountBlockchainFromUint64(0)
	if len(destinations) > 0 {
		to = destinations[0].Address
		amount = destinations[0].Amount
	}
	errMsg := ""
	if !tx.Success {
		errMsg = "transaction failed"
		if tx.VmStatus != "" {
			errMsg = tx.VmStatus
		}
	}

	return txinfo.LegacyTxInfo{
		To:            to,
		From:          xc.Address(tx.Sender),
		Amount:        amount,
		Sources:       sources,
		Destinations:  destinations,
		Fee:           feeu256,
		FeePayer:      xc.Address(feePayerAddress),
		Confirmations: int64(confirmations),
		BlockHash:     fmt.Sprintf("%d", tx.Version),
		// convert usec to sec
		BlockTime:  int64((tx.Timestamp / 1000) / 1000),
		TxID:       tx.Hash,
		BlockIndex: int64(block.BlockHeight),
		Error:      errMsg,
	}, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHashStr := args.TxHash()
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// remap to new tx
	return txinfo.TxInfoFromLegacy(chain, legacyTx, txinfo.Utxo), nil
}

// FetchBalance fetches balance for an Aptos address
func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	if contract, ok := args.Contract(); ok {
		balance, err := client.AptosClient.GetAccountBalance(string(args.Address()), string(contract), 0)
		if err != nil {
			return xc.NewAmountBlockchainFromUint64(0), err
		}
		return xc.AmountBlockchain(*balance), err
	} else {
		return client.FetchNativeBalance(ctx, args.Address())
	}
}

// FetchNativeBalance fetches the native asset balance for an Aptos address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	balance, err := client.AptosClient.GetAccountBalance(string(address), "0x1::aptos_coin::AptosCoin", 0)
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
		return zero, fmt.Errorf("the chain is too young")
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

type FungibleAssetMetadata struct {
	Decimals   int    `json:"decimals"`
	IconUri    string `json:"icon_uri"`
	Name       string `json:"name"`
	ProjectUri string `json:"project_uri"`
	Symbol     string `json:"symbol"`
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}
	// Try new fungible asset metadata standard first
	resp, err := client.AptosClient.GetAccountResource(string(contract), "0x1::fungible_asset::Metadata", 0)
	if err != nil {
		// Legacy coin info
		info, err2 := client.AptosClient.GetCoinInfo(string(contract))
		if err2 != nil {
			return 0, fmt.Errorf("could not get coin info: %v; or metadata: %v", err, err2)
		}
		return info.Decimals, nil
	} else {
		metadata := FungibleAssetMetadata{}
		err = reserialize(resp.Data, &metadata)
		if err != nil {
			return 0, fmt.Errorf("could not deserialize fungible_asset metadata: %v", err)
		}
		return metadata.Decimals, nil
	}
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	height, ok := args.Height()
	if !ok {
		ledger, err := client.AptosClient.LedgerInfo()
		if err != nil {
			return nil, err
		}
		height = ledger.BlockHeight
	}

	aptosBlock, err := client.AptosClient.GetBlockByHeight(fmt.Sprint(height), true)
	if err != nil {
		return nil, err
	}
	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			aptosBlock.BlockHeight,
			aptosBlock.BlockHash,
			time.Unix(int64(aptosBlock.BlockTimestamp/1000/1000), 0),
		),
	}
	for _, tx := range aptosBlock.Transactions {
		block.TransactionIds = append(block.TransactionIds, tx.Hash)
	}
	return block, nil
}
