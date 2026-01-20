package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/quicknode_blockbook"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/rest"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/rpc"
	"github.com/cordialsys/crosschain/chain/bitcoin/client/types"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"

	xclient "github.com/cordialsys/crosschain/client"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

const BlockbookFull string = "full-blockbook"

type BlockbookClient struct {
	Asset    *xc.ChainConfig
	Chaincfg *chaincfg.Params
	decoder  address.AddressDecoder

	skipAmountFilter bool

	bbClient types.BitcoinClientDriver
}

var _ xclient.Client = &BlockbookClient{}
var _ xclient.MultiTransferClient = &BlockbookClient{}
var _ address.WithAddressDecoder = &BlockbookClient{}

func NewJsonRpcClient(cfgI *xc.ChainConfig) (*BlockbookClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	httpClient := cfg.DefaultHttpClient()
	chaincfg, err := params.GetParams(cfg.Base())
	if err != nil {
		return &BlockbookClient{}, err
	}

	decoder := address.NewAddressDecoder()

	rpcClient := rpc.NewClient(cfg, &chaincfg)
	rpcClient.SetHttpClient(httpClient)

	return &BlockbookClient{
		Asset:            asset,
		Chaincfg:         &chaincfg,
		decoder:          decoder,
		skipAmountFilter: false,
		bbClient:         rpcClient,
	}, nil
}

func NewBlockbookClient(cfgI *xc.ChainConfig) (*BlockbookClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	httpClient := cfg.DefaultHttpClient()
	chaincfg, err := params.GetParams(cfg.Base())
	if err != nil {
		return &BlockbookClient{}, err
	}

	decoder := address.NewAddressDecoder()

	// Use REST blockboook client
	bbRest := rest.NewClient(cfg.URL)
	bbRest.SetHttpClient(httpClient)

	return &BlockbookClient{
		Asset:            asset,
		Chaincfg:         &chaincfg,
		decoder:          decoder,
		skipAmountFilter: false,
		bbClient:         bbRest,
	}, nil
}

func NewQuicknodeBlockbookClient(cfgI *xc.ChainConfig) (*BlockbookClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	httpClient := cfg.DefaultHttpClient()
	chaincfg, err := params.GetParams(cfg.Base())
	if err != nil {
		return &BlockbookClient{}, err
	}

	decoder := address.NewAddressDecoder()

	// Use REST blockboook client
	quicknodeBb := quicknode_blockbook.NewClient(cfg.URL)
	quicknodeBb.SetHttpClient(httpClient)

	return &BlockbookClient{
		Asset:            asset,
		Chaincfg:         &chaincfg,
		decoder:          decoder,
		skipAmountFilter: false,
		bbClient:         quicknodeBb,
	}, nil
}

func (client *BlockbookClient) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq) error {
	serial, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("bad tx: %v", err)
	}
	_, err = client.bbClient.SubmitTx(ctx, serial)
	return err
}

func (txBuilder *BlockbookClient) WithAddressDecoder(decoder address.AddressDecoder) address.WithAddressDecoder {
	txBuilder.decoder = decoder
	return txBuilder
}

func (client *BlockbookClient) UnspentOutputs(ctx context.Context, addr xc.Address) ([]tx_input.Output, error) {
	var formattedAddr string = string(addr)
	if client.Asset.GetChain().Chain == xc.BCH {
		if !strings.HasPrefix(string(addr), types.BitcoinCashPrefix) {
			formattedAddr = fmt.Sprintf("%s%s", types.BitcoinCashPrefix, addr)
		}
	}

	data, err := client.bbClient.ListUtxo(ctx, formattedAddr, client.Asset.GetChain().ConfirmedUtxo)
	if err != nil {
		return nil, err
	}

	// TODO try filtering using confirmed UTXO only for target amount, using heuristic as fallback.
	data = tx_input.FilterUnconfirmedHeuristic(data)
	btcAddr, err := client.decoder.Decode(addr, client.Chaincfg)
	if err != nil {
		return nil, fmt.Errorf("could not decode address: %v", err)
	}
	script, err := client.decoder.PayToAddrScript(btcAddr)
	if err != nil {
		return nil, fmt.Errorf("could not create pay-to-addr-script: %v", err)
	}

	outputs := tx_input.NewOutputs(data, script, addr)

	return outputs, nil
}

func (client *BlockbookClient) EstimateTotalFeeZcash(ctx context.Context, numActions int) (xc.AmountBlockchain, error) {
	// Zcash specifies fee in zatoshis per action.
	// An action is basically creating a utxo.
	defaultPrice := client.Asset.GetChain().ChainGasPriceDefault
	if defaultPrice < 1 {
		defaultPrice = 10000
	}
	return xc.NewAmountBlockchainFromUint64(uint64(defaultPrice) * uint64(numActions)), nil
}

func (client *BlockbookClient) EstimateSatsPerByteFee(ctx context.Context) (xc.AmountHumanReadable, error) {
	// Used by estimatefee RPC (number of future blocks to try to get mined within).  Smaller is more aggressive.
	numBlocksToGetMined := 2
	feeRate, err := client.bbClient.EstimateFee(ctx, numBlocksToGetMined)
	if err != nil {
		return xc.AmountHumanReadable{}, fmt.Errorf("failed to estimate fee: %w", err)
	}

	switch feeRate.Type {
	case types.FeeEstimationPerKb:
		btcPerKb := feeRate.Fee.Decimal()
		// convert to BTC/byte
		BtcPerB := btcPerKb.Div(decimal.NewFromInt(1000))
		// convert to sats/byte
		decimalFactor := decimal.NewFromInt32(10).Pow(decimal.NewFromInt32(client.Asset.GetChain().Decimals))
		satsPerB := xc.AmountHumanReadable(BtcPerB).Decimal().Mul(decimalFactor)

		satsPerByte := tx_input.LegacyFeeFilter(client.Asset.GetChain(), satsPerB.InexactFloat64(), client.Asset.GetChain().ChainGasMultiplier, client.Asset.GetChain().ChainMaxGasPrice)

		return xc.NewAmountHumanReadableFromFloat(satsPerByte), nil
	case types.FeeEstimationAverage:
		return feeRate.Fee, nil
	default:
		return xc.AmountHumanReadable{}, fmt.Errorf("unsupported FeeEstimation type: %s", feeRate.Type)
	}
}

func (client *BlockbookClient) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	txWithInfo := &txinfo.LegacyTxInfo{
		Amount: xc.NewAmountBlockchainFromUint64(0),
	}

	expectedTo := ""

	data, err := client.bbClient.GetTx(ctx, string(txHash))
	if err != nil {
		if apiErr, ok := err.(*types.ErrorResponse); ok {
			// they don't use 404 code :/
			if apiErr.HttpStatus >= 400 && strings.Contains(apiErr.ErrorMessage, "not found") {
				return *txWithInfo, errors.TransactionNotFoundf("%v", err)
			}

		}
		if apiErr, ok := err.(*types.JsonRPCError); ok {
			if strings.Contains(strings.ToLower(apiErr.Message), "no such mempool or blockchain transaction") {
				return *txWithInfo, errors.TransactionNotFoundf("%v", err)
			}
		}
		return *txWithInfo, err
	}

	latestBlock, err := client.bbClient.LatestBlock(ctx)
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

	sources := []*txinfo.LegacyTxInfoEndpoint{}
	destinations := []*txinfo.LegacyTxInfoEndpoint{}

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

	totalIn := xc.NewAmountBlockchainFromUint64(0)
	totalOut := xc.NewAmountBlockchainFromUint64(0)

	for _, in := range data.Vin {
		hash, _ := hex.DecodeString(in.TxID)
		totalIn = totalIn.Add(&in.Value)

		input := tx.Input{
			Output: tx_input.Output{
				Outpoint: tx_input.Outpoint{
					Hash:  hash,
					Index: uint32(in.Vout),
				},
				Value: in.Value,
			},
		}
		if len(in.Addresses) > 0 {
			input.Address = xc.Address(in.Addresses[0])
		}

		txObject.UnspentOutputs = append(txObject.UnspentOutputs, input.Output)
		inputs = append(inputs, input)
		utxoId := NewUtxoId(xc.TxHash(in.TxID), in.Vout)
		sources = append(sources, &txinfo.LegacyTxInfoEndpoint{
			Address:         input.Address,
			Amount:          input.Value,
			ContractAddress: "",
			NativeAsset:     xc.NativeAsset(asset),
			Asset:           string(asset),
			Event:           txinfo.NewEvent(utxoId, txinfo.MovementVariantNative),
		})
	}

	for _, out := range data.Vout {
		recipient := tx.Recipient{
			Value: out.Value,
		}
		if len(out.Addresses) > 0 {
			recipient.To = xc.Address(out.Addresses[0])
		}
		txObject.Recipients = append(txObject.Recipients, recipient)
		totalOut = totalOut.Add(&out.Value)
	}

	// detect from, to, amount
	from, _ := tx.DetectFrom(inputs)
	to, amount, _ := txObject.DetectToAndAmount(from, expectedTo)
	for i, out := range data.Vout {
		utxoId := NewUtxoId(txHash, out.N)
		if len(out.Addresses) > 0 {
			addr := out.Addresses[0]
			endpoint := &txinfo.LegacyTxInfoEndpoint{
				Address:     xc.Address(addr),
				Amount:      out.Value,
				NativeAsset: xc.NativeAsset(asset),
				Asset:       string(asset),
				Event:       txinfo.NewEvent(utxoId, txinfo.MovementVariantNative),
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
	txWithInfo.Fee = totalIn.Sub(&totalOut)

	return *txWithInfo, nil
}

func (client *BlockbookClient) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, args.TxHash())
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// delete the fee to avoid double counting.
	// the new model will calculate fees from the difference of inflows/outflows
	legacyTx.Fee = xc.NewAmountBlockchainFromUint64(0)

	// include change movements for non-legacy
	legacyTx.Destinations = legacyTx.GetDroppedBtcDestinations()

	// remap to new tx
	return txinfo.TxInfoFromLegacy(chain, legacyTx, txinfo.Utxo), nil
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
	if client.Asset.GetChain().Chain == xc.ZEC {
		totalFee, err := client.EstimateTotalFeeZcash(ctx, 2)
		if err != nil {
			return input, err
		}
		input.EstimatedTotalSize = totalFee
	} else {
		gasPerByte, err := client.EstimateSatsPerByteFee(ctx)
		if err != nil {
			return input, err
		}
		input.GasPricePerByteV2 = gasPerByte
		input.XGasPricePerByte = gasPerByte.ToBlockchain(0)
		if input.XGasPricePerByte.IsZero() {
			input.XGasPricePerByte = xc.NewAmountBlockchainFromUint64(1)
		}
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
	if client.Asset.GetChain().Chain == xc.ZEC {
		totalFee, err := client.EstimateTotalFeeZcash(ctx, len(args.Receivers())*2)
		if err != nil {
			return multiInput, err
		}
		multiInput.EstimatedTotalSize = totalFee
	} else {
		gasPerByte, err := client.EstimateSatsPerByteFee(ctx)
		multiInput.GasPricePerByte = gasPerByte
		if err != nil {
			return multiInput, err
		}
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
	args, _ := xcbuilder.NewTransferArgs(client.Asset.GetChain().Base(), from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *BlockbookClient) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	return 0, fmt.Errorf("unsupported")
}

func (client *BlockbookClient) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	height, ok := args.Height()
	if !ok {
		lastestHeight, err := client.bbClient.LatestBlock(ctx)
		if err != nil {
			return nil, err
		}
		height = lastestHeight
	}

	blockResponse, err := client.bbClient.GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			uint64(blockResponse.Height),
			blockResponse.Hash,
			time.Unix(blockResponse.Time, 0),
		),
	}
	block.TransactionIds = append(block.TransactionIds, blockResponse.GetTxIds()...)
	return block, nil
}
