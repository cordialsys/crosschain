package bitcoin

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

	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	log "github.com/sirupsen/logrus"
)

// Client for Bitcoin
type BlockchairClient struct {
	opts            ClientOptions
	httpClient      http.Client
	Asset           xc.ITask
	EstimateGasFunc xclient.EstimateGasFunc
}

var _ xclient.FullClientWithGas = &BlockchairClient{}

// NewClient returns a new Bitcoin Client
func NewBlockchairClient(cfgI xc.ITask) (*BlockchairClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	opts := DefaultClientOptions()
	httpClient := http.Client{}
	httpClient.Timeout = opts.Timeout
	params, err := GetParams(cfg)
	if err != nil {
		return &BlockchairClient{}, err
	}
	opts.Chaincfg = params
	opts.Host = cfg.URL
	opts.Password = cfg.AuthSecret
	if strings.TrimSpace(cfg.AuthSecret) == "" {
		return &BlockchairClient{}, fmt.Errorf("api token required for blockchair blockchain client")
	}
	return &BlockchairClient{
		opts:       opts,
		httpClient: httpClient,
		Asset:      asset,
	}, nil
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

	postUrl := fmt.Sprintf("%s/push/transaction?key=%s", client.opts.Host, client.opts.Password)
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

func (client *BlockchairClient) UnspentOutputs(ctx context.Context, minConf, maxConf int64, addr xc.Address) ([]Output, error) {
	var data blockchairAddressData
	res := []Output{}

	_, err := client.send(ctx, &data, "/dashboards/address", string(addr))
	if err != nil {
		return res, err
	}

	addressScript, _ := hex.DecodeString(data.Address.ScriptHex)

	// We calculate a threshold of 5% of the total BTC balance
	// To skip including small valued UTXO as part of the total utxo set.
	// This is done to avoid the case of including a UTXO from some tx with a very low
	// fee and making this TX get stuck.  However we'll still include our own remainder
	// UTXO's or large valued (>5%) UTXO's.

	// TODO a better way to do this would be to do during `.SetAmount` on the txInput,
	// So we can filter exactly for the target amount we need to send.
	oneBtc := uint64(1 * 100_000_000)
	totalSats := uint64(0)
	for _, u := range data.Utxo {
		totalSats += u.Value
	}
	threshold := uint64(0)
	if totalSats > oneBtc {
		threshold = (totalSats * 5) / 100
	}
	for _, u := range data.Utxo {
		if u.Block <= 0 && u.Value < threshold {
			// do not permit small-valued unconfirmed UTXO
			continue
		}
		hash, _ := hex.DecodeString(u.TxHash)
		// reverse
		for i, j := 0, len(hash)-1; i < j; i, j = i+1, j-1 {
			hash[i], hash[j] = hash[j], hash[i]
		}
		output := Output{
			Outpoint: Outpoint{
				Hash:  hash,
				Index: u.Index,
			},
			Value:        xc.NewAmountBlockchainFromUint64(u.Value),
			PubKeyScript: addressScript,
		}
		res = append(res, output)
	}

	return res, nil
}

func (client *BlockchairClient) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	allUnspentOutputs, err := client.UnspentOutputs(ctx, 0, 999999999, address)
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

// FetchTxInput returns tx input for a Bitcoin tx
func (client *BlockchairClient) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	input := NewTxInput()
	allUnspentOutputs, err := client.UnspentOutputs(ctx, 0, 999999999, xc.Address(from))
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

type blockchairStatsData struct {
	Blocks       uint64  `json:"blocks"`
	SuggestedFee float64 `json:"suggested_transaction_fee_per_byte_sat"`
}

type blockchairStats struct {
	Data    blockchairStatsData `json:"data"`
	Context BlockchairContext   `json:"context"`
}

type blockchairAddressFull struct {
	ScriptHex string `json:"script_hex"`
	Balance   uint64 `json:"balance"`
}

type blockchairTransactionFull struct {
	Hash    string `json:"hash"`
	Time    string `json:"time"`
	Fee     uint64 `json:"fee"`
	BlockId int64  `json:"block_id"`
}

type blockchairUTXO struct {
	// BlockId uint64  `json:"block_id"`
	TxHash  string `json:"transaction_hash"`
	Index   uint32 `json:"index"`
	Value   uint64 `json:"value"`
	Address string `json:"address,omitempty"`
	// This will be >0 if the UTXO is confirmed
	Block int64 `json:"block_id"`
}

type blockchairOutput struct {
	blockchairUTXO
	Recipient string `json:"recipient"`
	ScriptHex string `json:"script_hex"`
}

type blockchairInput struct {
	blockchairOutput
}

type BlockchairContext struct {
	Code  int32  `json:"code"` // 200 = ok
	Error string `json:"error,omitempty"`
	State int64  `json:"state"` // to count confirmations
}

type blockchairTransactionData struct {
	Transaction blockchairTransactionFull `json:"transaction"`
	Inputs      []blockchairInput         `json:"inputs"`
	Outputs     []blockchairOutput        `json:"outputs"`
}

type blockchairAddressData struct {
	// Transactions []blockchairTransaction `json:"transactions"`
	Address blockchairAddressFull `json:"address"`
	Utxo    []blockchairUTXO      `json:"utxo"`
}

type blockchairData struct {
	Data    map[string]json.RawMessage `json:"data"`
	Context BlockchairContext          `json:"context"`
}
type blockchairNotFoundData struct {
	Data    []string          `json:"data"`
	Context BlockchairContext `json:"context"`
}

func (client *BlockchairClient) send(ctx context.Context, resp interface{}, method string, params ...string) (*BlockchairContext, error) {
	url := fmt.Sprintf("%s%s?key=%s", client.opts.Host, method, client.opts.Password)
	if len(params) > 0 {
		value := params[0]
		url = fmt.Sprintf("%s%s/%s?key=%s", client.opts.Host, method, value, client.opts.Password)
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
	timestamp, _ := time.Parse("2006-01-02 15:04:05", data.Transaction.Time)
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
	tx := &Tx{
		Input:      *NewTxInput(),
		Recipients: []Recipient{},
		MsgTx:      &wire.MsgTx{},
		Signed:     true,
	}
	inputs := []Input{}
	// btc chains the native asset and asset are the same
	asset := client.Asset.GetChain().Chain

	for _, in := range data.Inputs {
		hash, _ := hex.DecodeString(in.TxHash)
		// sigScript, _ := hex.DecodeString(in.ScriptHex)

		input := Input{
			Output: Output{
				Outpoint: Outpoint{
					Hash:  hash,
					Index: in.Index,
				},
				Value: xc.NewAmountBlockchainFromUint64(in.Value),
				// PubKeyScript: []byte{},
			},
			// SigScript: sigScript,
			Address: xc.Address(in.Recipient),
		}
		tx.Input.UnspentOutputs = append(tx.Input.UnspentOutputs, input.Output)
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
		recipient := Recipient{
			To:    xc.Address(out.Recipient),
			Value: xc.NewAmountBlockchainFromUint64(out.Value),
		}
		tx.Recipients = append(tx.Recipients, recipient)

	}

	// detect from, to, amount
	from, _ := DetectFrom(inputs)
	to, amount, _ := tx.DetectToAndAmount(from, expectedTo)
	for _, out := range data.Outputs {
		if out.Recipient != from {
			destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
				Address:     xc.Address(out.Recipient),
				Amount:      xc.NewAmountBlockchainFromUint64(out.Value),
				NativeAsset: xc.NativeAsset(asset),
				Asset:       string(asset),
			})
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

// EstimateGas(ctx context.Context) (AmountBlockchain, error)
func (client *BlockchairClient) RegisterEstimateGasCallback(estimateGas xclient.EstimateGasFunc) {
	client.EstimateGasFunc = estimateGas
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

	// Min 10 sats/byte
	if satsPerByteFloat < 10 {
		satsPerByteFloat = 10
	}
	// add 50% extra default
	defaultMultiplier := 1.5
	multiplier := client.Asset.GetChain().ChainGasMultiplier
	if multiplier < 0.01 {
		multiplier = defaultMultiplier
	}

	satsPerByteFloat *= multiplier

	max := client.Asset.GetChain().ChainMaxGasPrice
	if max < 0.01 {
		// max 10k sats/byte
		max = 10000
	}
	if satsPerByteFloat > max {
		satsPerByteFloat = max
	}
	satsPerByte := uint64(satsPerByteFloat)

	return xc.NewAmountBlockchainFromUint64(satsPerByte), nil
}
