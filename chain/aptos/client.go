package aptos

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/coming-chat/go-aptos/aptosclient"
	"github.com/coming-chat/go-aptos/aptostypes"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/aptos/events"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
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
		zero := [32]byte{}
		privateKey := ed25519.NewKeyFromSeed(zero[:])
		privateKey.Public()

		txI, err := builder.Transfer(args, input)
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not create tx: %v", err)
		}
		tx := txI.(*Tx)

		hashes, err := tx.Sighashes()
		if err != nil {
			return &tx_input.TxInput{}, fmt.Errorf("could not get sighashes: %v", err)
		}
		signatureData := ed25519.Sign(privateKey, hashes[0].Payload)
		tx.SetSignatures(&xc.SignatureResponse{
			Signature: signatureData,
			PublicKey: pubkey,
			Address:   args.GetFrom(),
		})

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
			log = log.WithField("status", output[0].VmStatus)
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

	return input, nil
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
		return fmt.Errorf("could not serialize tx: %v", err)
	}
	newTxn, err := client.AptosClient.SubmitSignedBCSTransaction(tx_bz)
	_ = newTxn
	return err
}

// FetchLegacyTxInfo returns tx info for a Aptos tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	tx, err := client.AptosClient.GetTransactionByHash(string(txHash))
	if err != nil {
		if aptosErr, ok := err.(*aptostypes.RestError); ok {
			if aptosErr.Code == http.StatusNotFound {
				return xclient.LegacyTxInfo{}, errors.TransactionNotFoundf("%v", err)
			}
		}
		return xclient.LegacyTxInfo{}, err
	}

	block, err := client.AptosClient.GetBlockByVersion(fmt.Sprintf("%d", tx.Version), false)
	if err != nil {
		return xclient.LegacyTxInfo{}, err
	}
	ledger, err := client.AptosClient.LedgerInfo()
	if err != nil {
		return xclient.LegacyTxInfo{}, err
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
		return xclient.LegacyTxInfo{}, err
	}

	chainCfg := client.Asset.GetChain()
	// Legacy behavior expects that ContractAddress is blank for Aptos native asset -- this is not done
	// for new txinfo endpoint.
	for _, endpoint := range sources {
		if endpoint.ContractAddress == xc.ContractAddress(chainCfg.ChainCoin) {
			endpoint.ContractId = endpoint.ContractAddress
			endpoint.ContractAddress = xc.ContractAddress(chainCfg.Chain)
		}
	}
	for _, endpoint := range destinations {
		if endpoint.ContractAddress == xc.ContractAddress(chainCfg.ChainCoin) {
			endpoint.ContractId = endpoint.ContractAddress
			endpoint.ContractAddress = xc.ContractAddress(chainCfg.Chain)
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

	return xclient.LegacyTxInfo{
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

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Utxo), nil
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

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	info, err := client.AptosClient.GetCoinInfo(string(contract))
	if err != nil {
		return 0, nil
	}
	return info.Decimals, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
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
	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(
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
