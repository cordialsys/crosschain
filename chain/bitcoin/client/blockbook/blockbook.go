package blockbook

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/builder"
	clientcommon "github.com/cordialsys/crosschain/chain/bitcoin/client"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/client/errors"

	// "github.com/cordialsys/crosschain/chain/bitcoin_cash"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

type BlockbookClient struct {
	httpClient http.Client
	Asset      xc.ITask
	Chaincfg   *chaincfg.Params
	Url        string
	decoder    address.AddressDecoder

	skipAmountFilter bool
}

var _ xclient.Client = &BlockbookClient{}
var _ xclient.MultiTransferClient = &BlockbookClient{}
var _ address.WithAddressDecoder = &BlockbookClient{}

func NewClient(cfgI xc.ITask) (*BlockbookClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	httpClient := cfg.DefaultHttpClient()
	chaincfg, err := params.GetParams(cfg.Base())
	if err != nil {
		return &BlockbookClient{}, err
	}
	url := cfg.URL
	url = strings.TrimSuffix(url, "/")
	decoder := address.NewAddressDecoder()

	return &BlockbookClient{
		*httpClient,
		asset,
		chaincfg,
		url,
		decoder,
		false,
	}, nil
}

func (client *BlockbookClient) LatestBlock(ctx context.Context) (uint64, error) {
	var stats StatsResponse

	err := client.get(ctx, "/api/v2", &stats)
	if err != nil {
		return 0, err
	}

	return uint64(stats.Backend.Blocks), nil
}

func (client *BlockbookClient) SubmitTx(ctx context.Context, tx xc.Tx) error {
	serial, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("bad tx: %v", err)
	}

	postData := hex.EncodeToString(serial)
	err = client.post(ctx, "/api/v2/sendtx/", "text/plain", []byte(postData), nil)
	if err != nil {
		return err
	}

	return nil
}

func (txBuilder *BlockbookClient) WithAddressDecoder(decoder address.AddressDecoder) address.WithAddressDecoder {
	txBuilder.decoder = decoder
	return txBuilder
}

const BitcoinCashPrefix = "bitcoincash:"

func (client *BlockbookClient) UnspentOutputs(ctx context.Context, addr xc.Address) ([]tx_input.Output, error) {
	var data UtxoResponse
	var formattedAddr string = string(addr)
	if client.Asset.GetChain().Chain == xc.BCH {
		if !strings.HasPrefix(string(addr), BitcoinCashPrefix) {
			formattedAddr = fmt.Sprintf("%s%s", BitcoinCashPrefix, addr)
		}
	}

	err := client.get(ctx, fmt.Sprintf("api/v2/utxo/%s", formattedAddr), &data)
	if err != nil {
		return nil, err
	}

	// TODO try filtering using confirmed UTXO only for target amount, using heuristic as fallback.
	data = tx_input.FilterUnconfirmedHeuristic(data)
	btcAddr, err := client.decoder.Decode(addr, client.Chaincfg)
	if err != nil {
		return nil, err
	}
	script, err := txscript.PayToAddrScript(btcAddr)
	if err != nil {
		return nil, err
	}

	outputs := tx_input.NewOutputs(data, script, addr)

	return outputs, nil
}

func (client *BlockbookClient) EstimateFee(ctx context.Context) (xc.AmountBlockchain, error) {
	var data EstimateFeeResponse
	// fee estimate for last N blocks
	blocks := 6
	err := client.get(ctx, fmt.Sprintf("/api/v2/estimatefee/%d", blocks), &data)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}

	btcPerKb, err := decimal.NewFromString(data.Result)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	// convert to BTC/byte
	BtcPerB := btcPerKb.Div(decimal.NewFromInt(1000))
	// convert to sats/byte
	satsPerB := xc.AmountHumanReadable(BtcPerB).ToBlockchain(client.Asset.GetDecimals())

	satsPerByte := tx_input.LegacyFeeFilter(client.Asset.GetChain(), satsPerB.Uint64(), client.Asset.GetChain().ChainGasMultiplier, client.Asset.GetChain().ChainMaxGasPrice)

	return xc.NewAmountBlockchainFromUint64(satsPerByte), nil
}

func (client *BlockbookClient) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.LegacyTxInfo, error) {
	var data TransactionResponse
	txWithInfo := &xclient.LegacyTxInfo{
		Amount: xc.NewAmountBlockchainFromUint64(0), // prevent nil pointer exception
		Fee:    xc.NewAmountBlockchainFromUint64(0),
	}

	expectedTo := ""

	err := client.get(ctx, "/api/v2/tx/"+string(txHash), &data)
	if err != nil {
		if bbErr, ok := err.(*ErrorResponse); ok {
			// they don't use 404 code :/
			if bbErr.HttpStatus >= 400 && strings.Contains(bbErr.ErrorMessage, "not found") {
				return *txWithInfo, errors.TransactionNotFoundf("%v", err)
			}
		}
		return *txWithInfo, err
	}

	latestBlock, err := client.LatestBlock(ctx)
	if err != nil {
		return *txWithInfo, err
	}

	txWithInfo.Fee = xc.NewAmountBlockchainFromStr(data.Fees)
	timestamp := time.Unix(data.BlockTime, 0)
	if data.BlockHeight > 0 {
		txWithInfo.BlockTime = timestamp.Unix()
		txWithInfo.BlockIndex = int64(data.BlockHeight)
		txWithInfo.BlockHash = data.BlockHash
		txWithInfo.Confirmations = int64(latestBlock) - int64(data.BlockHeight) + 1
		txWithInfo.Status = xc.TxStatusSuccess
	} else {
		// still in mempool
	}
	txWithInfo.TxID = string(txHash)

	sources := []*xclient.LegacyTxInfoEndpoint{}
	destinations := []*xclient.LegacyTxInfoEndpoint{}

	// build Tx
	txObject := &tx.Tx{
		UnspentOutputs: []tx_input.Output{},
		Recipients:     []tx.Recipient{},
		MsgTx:          &wire.MsgTx{},
		Signed:         true,
	}
	inputs := []tx.Input{}
	// btc chains the native asset and asset are the same
	asset := client.Asset.GetChain().Chain

	for _, in := range data.Vin {
		hash, _ := hex.DecodeString(in.TxID)
		// sigScript, _ := hex.DecodeString(in.ScriptHex)

		input := tx.Input{
			Output: tx_input.Output{
				Outpoint: tx_input.Outpoint{
					Hash:  hash,
					Index: uint32(in.Vout),
				},
				Value: xc.NewAmountBlockchainFromStr(in.Value),
				// PubKeyScript: []byte{},
			},
			// SigScript: sigScript,
			// Address: xc.Address(in.Addresses[0]),
		}
		if len(in.Addresses) > 0 {
			input.Address = xc.Address(in.Addresses[0])
		}

		txObject.UnspentOutputs = append(txObject.UnspentOutputs, input.Output)
		inputs = append(inputs, input)
		utxoId := clientcommon.NewUtxoId(xc.TxHash(in.TxID), in.Vout)
		sources = append(sources, &xclient.LegacyTxInfoEndpoint{
			Address:         input.Address,
			Amount:          input.Value,
			ContractAddress: "",
			NativeAsset:     xc.NativeAsset(asset),
			Asset:           string(asset),
			Event:           xclient.NewEvent(utxoId, xclient.MovementVariantNative),
		})
	}

	for _, out := range data.Vout {
		recipient := tx.Recipient{
			// To:    xc.Address(out.Recipient),
			Value: xc.NewAmountBlockchainFromStr(out.Value),
		}
		if len(out.Addresses) > 0 {
			recipient.To = xc.Address(out.Addresses[0])
		}
		txObject.Recipients = append(txObject.Recipients, recipient)
	}

	// detect from, to, amount
	from, _ := tx.DetectFrom(inputs)
	to, amount, _ := txObject.DetectToAndAmount(from, expectedTo)
	for i, out := range data.Vout {
		utxoId := clientcommon.NewUtxoId(txHash, out.N)
		if len(out.Addresses) > 0 {
			addr := out.Addresses[0]
			endpoint := &xclient.LegacyTxInfoEndpoint{
				Address:     xc.Address(addr),
				Amount:      xc.NewAmountBlockchainFromStr(out.Value),
				NativeAsset: xc.NativeAsset(asset),
				Asset:       string(asset),
				Event:       xclient.NewEvent(utxoId, xclient.MovementVariantNative),
			}
			if addr != from {
				// legacy endpoint drops 'change' movements
				destinations = append(destinations, endpoint)
			}
			txWithInfo.AddDroppedDestination(i, endpoint)
		}
	}

	// from
	// to
	// amount
	txWithInfo.From = xc.Address(from)
	txWithInfo.To = xc.Address(to)
	txWithInfo.Amount = amount
	txWithInfo.Sources = sources
	txWithInfo.Destinations = destinations

	return *txWithInfo, nil
}

func (client *BlockbookClient) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// delete the fee to avoid double counting.
	// the new model will calculate fees from the difference of inflows/outflows
	legacyTx.Fee = xc.NewAmountBlockchainFromUint64(0)

	// include change movements for non-legacy
	legacyTx.Destinations = legacyTx.GetDroppedBtcDestinations()

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Utxo), nil
}

func (client *BlockbookClient) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	allUnspentOutputs, err := client.UnspentOutputs(ctx, args.Address())
	amount := xc.NewAmountBlockchainFromUint64(0)
	if err != nil {
		return amount, err
	}
	for _, unspent := range allUnspentOutputs {
		amount = amount.Add(&unspent.Value)
	}
	return amount, nil
}

func (client *BlockbookClient) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()
	allUnspentOutputs, err := client.UnspentOutputs(ctx, args.GetFrom())
	if err != nil {
		return input, err
	}
	input.Address = args.GetFrom()
	input.UnspentOutputs = allUnspentOutputs
	gasPerByte, err := client.EstimateFee(ctx)
	input.GasPricePerByte = gasPerByte
	if err != nil {
		return input, err
	}

	input.EstimatedSizePerSpentUtxo = tx_input.PerUtxoSizeEstimate(client.Asset.GetChain())

	// Filter the UTXO only if the amount is explicitly passed (otherwise we return all UTXOs)
	if !client.skipAmountFilter && args.GetAmount().Uint64() > 1 {
		// filter the UTXO set needed
		input.SetAmount(args.GetAmount())
	}

	return input, nil
}

func (client *BlockbookClient) FetchMultiTransferInput(ctx context.Context, args xcbuilder.MultiTransferArgs) (xc.MultiTransferInput, error) {
	multiInput := tx_input.NewMultiTransferInput()

	// Fetch all UTXO from all spenders in a flat array
	allUtxo := []tx_input.Output{}
	for _, spender := range args.Spenders() {
		utxo, err := client.UnspentOutputs(ctx, spender.GetFrom())
		if err != nil {
			return multiInput, err
		}
		allUtxo = append(allUtxo, utxo...)
	}

	// Calculate the total amount that will be distributed
	totalAmount := xc.NewAmountBlockchainFromUint64(0)
	for _, receiver := range args.Receivers() {
		amount := receiver.GetAmount()
		totalAmount = totalAmount.Add(&amount)
	}

	// Sort + Filter the UTXO for the minimum set (+ some ~10 extra) that satisfies the total amount
	filteredUtxo := tx_input.FilterForMinUtxoSet(allUtxo, totalAmount, 10)

	// Group back by address
	groupedUtxoByAddress := map[xc.Address][]tx_input.Output{}
	serialiedByAddress := map[xc.Address]bool{}
	for _, utxo := range filteredUtxo {
		groupedUtxoByAddress[utxo.Address] = append(groupedUtxoByAddress[utxo.Address], utxo)
	}
	// iterate over array instead of map to preserve order
	for _, utxo := range filteredUtxo {
		if _, ok := serialiedByAddress[utxo.Address]; !ok {
			multiInput.Inputs = append(multiInput.Inputs, tx_input.TxInput{
				UnspentOutputs: groupedUtxoByAddress[utxo.Address],
				Address:        utxo.Address,
			})
			serialiedByAddress[utxo.Address] = true
		}
	}
	for _, input := range multiInput.Inputs {
		for _, utxo := range input.UnspentOutputs {
			logrus.WithFields(logrus.Fields{
				"address": utxo.Address,
				"amount":  utxo.Value,
				"utxo":    utxo.Outpoint.String(),
				"total":   len(allUtxo),
			}).Debug("prioritized utxo")
		}
	}

	// Estimate fees
	gasPerByte, err := client.EstimateFee(ctx)
	multiInput.GasPricePerByte = gasPerByte
	if err != nil {
		return multiInput, err
	}
	size, err := client.EstimateTxSize(ctx, args, *multiInput)
	if err != nil {
		return multiInput, err
	}
	multiInput.EstimatedSize = uint64(size)

	return multiInput, nil
}

// Estimate the size of the transaction in bytes.  This is useful for estimating fees, and is more accurate than
// just counting the number of UTXO.
func (client *BlockbookClient) EstimateTxSize(ctx context.Context, args xcbuilder.MultiTransferArgs, input tx_input.MultiTransferInput) (int, error) {
	txBuilder, err := builder.NewTxBuilder(client.Asset.GetChain().Base())
	if err != nil {
		return 0, err
	}
	// ensure the fee estimate is 0 for the estimation
	input.EstimatedSize = 0

	tx, err := txBuilder.MultiTransfer(args, &input)
	if err != nil {
		return 0, err
	}

	sighashes, err := tx.Sighashes()
	if err != nil {
		return 0, err
	}
	sigs := []*xc.SignatureResponse{}
	for _, sighash := range sighashes {
		sigs = append(sigs, &xc.SignatureResponse{
			Signature: make([]byte, 64),
			PublicKey: make([]byte, 64),
			Address:   sighash.Signer,
		})
	}
	err = tx.SetSignatures(sigs...)
	if err != nil {
		return 0, err
	}
	serial, err := tx.Serialize()
	if err != nil {
		return 0, err
	}

	return len(serial), nil
}

func (client *BlockbookClient) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount, and we cannot
	// estimate the fee accurately without knowing the number of utxo we need to spend to satisfy the amount.
	client.skipAmountFilter = true
	defer func() {
		client.skipAmountFilter = false
	}()
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *BlockbookClient) get(ctx context.Context, path string, resp interface{}) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", client.Url, path)
	logrus.WithFields(logrus.Fields{
		"url": url,
	}).Debug("get")
	res, err := client.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("blockbook get failed: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		var errResponse ErrorResponse
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			return fmt.Errorf("failed to get %s: code=%d", path, res.StatusCode)
		}
		errResponse.HttpStatus = res.StatusCode
		return &errResponse
	}

	if resp != nil {
		err = json.Unmarshal(body, &resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *BlockbookClient) post(ctx context.Context, path string, contentType string, input []byte, resp interface{}) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", client.Url, path)
	logrus.WithFields(logrus.Fields{
		"url":  url,
		"body": string(input),
	}).Debug("post")
	res, err := client.httpClient.Post(url, contentType, bytes.NewReader(input))
	if err != nil {
		return fmt.Errorf("blockbook post failed: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		var errResponse ErrorResponse
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			return fmt.Errorf("failed to get %s: code=%d", path, res.StatusCode)
		}
		errResponse.HttpStatus = res.StatusCode
		return &errResponse
	}

	if resp != nil {
		err = json.Unmarshal(body, &resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *BlockbookClient) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	return 0, fmt.Errorf("unsupported")
}

func (client *BlockbookClient) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	height, ok := args.Height()
	if !ok {
		var stats StatsResponse

		err := client.get(ctx, "/api/v2", &stats)
		if err != nil {
			return nil, err
		}
		height = uint64(stats.Backend.Blocks)
	}

	var blockResponse Block
	err := client.get(ctx, fmt.Sprintf("/api/v2/block/%d", height), &blockResponse)
	if err != nil {
		return nil, err
	}

	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(
			client.Asset.GetChain().Chain,
			uint64(blockResponse.Height),
			blockResponse.Hash,
			time.Unix(blockResponse.Time, 0),
		),
	}
	for _, tx := range blockResponse.Txs {
		block.TransactionIds = append(block.TransactionIds, tx.TxID)
	}
	return block, nil
}
