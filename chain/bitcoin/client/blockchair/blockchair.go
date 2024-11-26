package blockchair

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	log "github.com/sirupsen/logrus"
)

// Client for Bitcoin
type BlockchairClient struct {
	// opts            ClientOptions
	httpClient     http.Client
	Asset          xc.ITask
	Chaincfg       *chaincfg.Params
	Url            string
	ApiKey         string
	addressDecoder address.AddressDecoder
}

var _ xclient.FullClient = &BlockchairClient{}
var _ xclient.ClientWithDecimals = &BlockchairClient{}
var _ address.WithAddressDecoder = &BlockchairClient{}

// NewClient returns a new Bitcoin Client
func NewBlockchairClient(cfgI xc.ITask) (*BlockchairClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	httpClient := http.Client{}
	params, err := params.GetParams(cfg)
	if err != nil {
		return &BlockchairClient{}, err
	}

	if strings.TrimSpace(cfg.AuthSecret) == "" {
		return &BlockchairClient{}, fmt.Errorf("api token required for blockchair blockchain client (set .auth reference)")
	}
	return &BlockchairClient{
		ApiKey:         cfg.AuthSecret,
		Url:            cfg.URL,
		Chaincfg:       params,
		httpClient:     httpClient,
		Asset:          asset,
		addressDecoder: &address.BtcAddressDecoder{},
	}, nil
}

func (txBuilder *BlockchairClient) WithAddressDecoder(decoder address.AddressDecoder) address.WithAddressDecoder {
	txBuilder.addressDecoder = decoder
	return txBuilder
}

func (client *BlockchairClient) LatestBlock(ctx context.Context) (uint64, error) {
	var stats blockchairStats

	_, err := client.send(ctx, &stats, "/stats")
	if err != nil {
		return 0, err
	}

	return stats.Data.Blocks, nil
}

func (client *BlockchairClient) SubmitTx(ctx context.Context, tx xc.Tx) error {
	serial, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("bad tx: %v", err)
	}

	postUrl := fmt.Sprintf("%s/push/transaction?key=%s", client.Url, client.ApiKey)
	postData := fmt.Sprintf("data=%s", hex.EncodeToString(serial))
	log.Debug(postData)
	res, err := client.httpClient.Post(postUrl, "application/x-www-form-urlencoded", bytes.NewBuffer([]byte(postData)))
	if err != nil {
		log.Warn(err)
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error(err)
		return err
	}

	var apiData blockchairData
	err = json.Unmarshal(body, &apiData)
	if err != nil {
		log.Error(err)
		log.Error(string(body))
		return err
	}

	if apiData.Context.Code != 200 {
		log.Error(string(body))
		return errors.New(apiData.Context.Error)
	}

	return nil
}

func (client *BlockchairClient) UnspentOutputs(ctx context.Context, addr xc.Address) ([]tx_input.Output, error) {
	var data blockchairAddressData
	res := []tx_input.Output{}

	_, err := client.send(ctx, &data, "/dashboards/address", string(addr))
	if err != nil {
		return res, err
	}

	addressScript, _ := hex.DecodeString(data.Address.ScriptHex)

	utxos := tx_input.FilterUnconfirmedHeuristic(data.Utxo)
	outputs := tx_input.NewOutputs(utxos, addressScript)

	return outputs, nil
}

func (client *BlockchairClient) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	allUnspentOutputs, err := client.UnspentOutputs(ctx, address)
	amount := xc.NewAmountBlockchainFromUint64(0)
	if err != nil {
		return amount, err
	}
	for _, unspent := range allUnspentOutputs {
		amount = amount.Add(&unspent.Value)
	}
	return amount, nil
}

func (client *BlockchairClient) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalance(ctx, address)
}

func (client *BlockchairClient) EstimateGasFee(ctx context.Context, numBlocks int64) (float64, error) {
	var stats blockchairStats

	_, err := client.send(ctx, &stats, "/stats")
	if err != nil {
		return 0, err
	}

	return float64(stats.Data.SuggestedFee), nil
}

func (client *BlockchairClient) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()
	allUnspentOutputs, err := client.UnspentOutputs(ctx, args.GetFrom())
	if err != nil {
		return input, err
	}
	input.UnspentOutputs = allUnspentOutputs
	gasPerByte, err := client.EstimateGas(ctx)
	input.GasPricePerByte = gasPerByte
	if err != nil {
		return input, err
	}

	return input, nil
}

func (client *BlockchairClient) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *BlockchairClient) send(ctx context.Context, resp interface{}, method string, params ...string) (*BlockchairContext, error) {
	url := fmt.Sprintf("%s%s?key=%s", client.Url, method, client.ApiKey)
	if len(params) > 0 {
		value := params[0]
		url = fmt.Sprintf("%s%s/%s?key=%s", client.Url, method, value, client.ApiKey)
	}

	res, err := client.httpClient.Get(url)
	if err != nil {
		log.Warn(err)
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	var apiData blockchairData
	err = json.Unmarshal(body, &apiData)
	if err != nil {
		var notFound blockchairNotFoundData
		err2 := json.Unmarshal(body, &notFound)
		if err2 == nil {
			return nil, errors.New("not found: could not find a result on blockchair")
		}
		log.Error(err)
		log.Error(string(body))
		return nil, err
	}
	// fmt.Println("<<", string(body))

	if apiData.Context.Code != 200 {
		return &apiData.Context, fmt.Errorf("error code failure: %d: %s", apiData.Context.Code, apiData.Context.Error)
	}

	if len(params) > 0 {
		value := params[0]
		innerData, found := apiData.Data[value]
		if !found {
			log.Error(err)
			log.Error(string(body))
			return nil, errors.New("invalid response format")
		}
		err = json.Unmarshal(innerData, resp)
	} else {
		err = json.Unmarshal(body, resp)
	}
	return &apiData.Context, err
}

func (client *BlockchairClient) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	var data blockchairTransactionData
	txWithInfo := &xc.LegacyTxInfo{
		Amount: xc.NewAmountBlockchainFromUint64(0), // prevent nil pointer exception
		Fee:    xc.NewAmountBlockchainFromUint64(0),
	}

	expectedTo := ""

	blockchairContext, err := client.send(ctx, &data, "/dashboards/transaction", string(txHash))
	if err != nil {
		return *txWithInfo, err
	}

	txWithInfo.Fee = xc.NewAmountBlockchainFromUint64(data.Transaction.Fee)
	timestamp, _ := time.Parse(time.DateTime, data.Transaction.Time)
	if data.Transaction.BlockId > 0 {
		txWithInfo.BlockTime = timestamp.Unix()
		txWithInfo.BlockIndex = data.Transaction.BlockId
		// txWithInfo.BlockHash = n/a
		txWithInfo.Confirmations = blockchairContext.State - data.Transaction.BlockId + 1
		txWithInfo.Status = xc.TxStatusSuccess
	}
	txWithInfo.TxID = data.Transaction.Hash

	sources := []*xc.LegacyTxInfoEndpoint{}
	destinations := []*xc.LegacyTxInfoEndpoint{}

	// build Tx
	txObject := &tx.Tx{
		Input:      tx_input.NewTxInput(),
		Recipients: []tx.Recipient{},
		MsgTx:      &wire.MsgTx{},
		Signed:     true,
	}
	inputs := []tx.Input{}
	// btc chains the native asset and asset are the same
	asset := client.Asset.GetChain().Chain

	for _, in := range data.Inputs {
		hash, _ := hex.DecodeString(in.TxHash)
		// sigScript, _ := hex.DecodeString(in.ScriptHex)

		input := tx.Input{
			Output: tx_input.Output{
				Outpoint: tx_input.Outpoint{
					Hash:  hash,
					Index: in.Index,
				},
				Value: xc.NewAmountBlockchainFromUint64(in.Value),
				// PubKeyScript: []byte{},
			},
			// SigScript: sigScript,
			Address: xc.Address(in.Recipient),
		}
		txObject.Input.UnspentOutputs = append(txObject.Input.UnspentOutputs, input.Output)
		inputs = append(inputs, input)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address:         input.Address,
			Amount:          input.Value,
			ContractAddress: "",
			NativeAsset:     xc.NativeAsset(asset),
			Asset:           string(asset),
		})
	}

	for _, out := range data.Outputs {
		recipient := tx.Recipient{
			To:    xc.Address(out.Recipient),
			Value: xc.NewAmountBlockchainFromUint64(out.Value),
		}
		txObject.Recipients = append(txObject.Recipients, recipient)

	}

	// detect from, to, amount
	from, _ := tx.DetectFrom(inputs)
	to, amount, _ := txObject.DetectToAndAmount(from, expectedTo)
	for _, out := range data.Outputs {
		endpoint := &xc.LegacyTxInfoEndpoint{
			Address:     xc.Address(out.Recipient),
			Amount:      xc.NewAmountBlockchainFromUint64(out.Value),
			NativeAsset: xc.NativeAsset(asset),
			Asset:       string(asset),
		}
		if out.Recipient != from {
			// legacy endpoint drops 'change' movements
			destinations = append(destinations, endpoint)
		} else {
			txWithInfo.AddDroppedDestination(endpoint)
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

func (client *BlockchairClient) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// delete the fee to avoid double counting.
	// the new model will calculate fees from the difference of inflows/outflows
	legacyTx.Fee = xc.NewAmountBlockchainFromUint64(0)

	// add back the change movements
	legacyTx.Destinations = append(legacyTx.Destinations, legacyTx.GetDroppedBtcDestinations()...)

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Utxo), nil
}

func (client *BlockchairClient) EstimateGas(ctx context.Context) (xc.AmountBlockchain, error) {
	// estimate using last 1 blocks
	numBlocks := 1
	fallbackGasPerByte := xc.NewAmountBlockchainFromUint64(10)
	satsPerByteFloat, err := client.EstimateGasFee(ctx, int64(numBlocks))

	if err != nil {
		return fallbackGasPerByte, err
	}

	if satsPerByteFloat <= 0.0 {
		return fallbackGasPerByte, fmt.Errorf("invalid sats per byte: %v", satsPerByteFloat)
	}

	satsPerByte := tx_input.LegacyFeeFilter(client.Asset.GetChain(), uint64(satsPerByteFloat), client.Asset.GetChain().ChainGasMultiplier, client.Asset.GetChain().ChainMaxGasPrice)

	return xc.NewAmountBlockchainFromUint64(satsPerByte), nil
}

func (client *BlockchairClient) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	return 0, fmt.Errorf("unsupported")
}
