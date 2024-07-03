package blockbook

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

type BlockbookClient struct {
	httpClient http.Client
	Asset      xc.ITask
	Chaincfg   *chaincfg.Params
	Url        string
}

var _ xclient.FullClient = &BlockbookClient{}

func NewClient(cfgI xc.ITask) (*BlockbookClient, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	httpClient := http.Client{}
	params, err := params.GetParams(cfg)
	if err != nil {
		return &BlockbookClient{}, err
	}
	url := cfg.URL
	url = strings.TrimSuffix(url, "/")

	return &BlockbookClient{
		Url:        url,
		Chaincfg:   params,
		httpClient: httpClient,
		Asset:      asset,
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

func (client *BlockbookClient) UnspentOutputs(ctx context.Context, addr xc.Address) ([]tx_input.Output, error) {
	var data UtxoResponse
	err := client.get(ctx, fmt.Sprintf("api/v2/utxo/%s", addr), &data)
	if err != nil {
		return nil, err
	}

	data = tx_input.FilterUnconfirmedHeuristic(data)
	outputs := tx_input.NewOutputs(data, []byte{})

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

	satsPerByte := tx_input.LegacyFeeFilter(satsPerB.Uint64(), client.Asset.GetChain().ChainGasMultiplier, client.Asset.GetChain().ChainMaxGasPrice)

	return xc.NewAmountBlockchainFromUint64(satsPerByte), nil
}

func (client *BlockbookClient) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	var data TransactionResponse
	txWithInfo := &xc.LegacyTxInfo{
		Amount: xc.NewAmountBlockchainFromUint64(0), // prevent nil pointer exception
		Fee:    xc.NewAmountBlockchainFromUint64(0),
	}

	expectedTo := ""

	err := client.get(ctx, "/api/v2/tx/"+string(txHash), &data)
	if err != nil {
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
	}
	txWithInfo.TxID = string(txHash)

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
	for _, out := range data.Vout {
		if len(out.Addresses) > 0 {
			addr := out.Addresses[0]
			endpoint := &xc.LegacyTxInfoEndpoint{
				Address:     xc.Address(addr),
				Amount:      xc.NewAmountBlockchainFromStr(out.Value),
				NativeAsset: xc.NativeAsset(asset),
				Asset:       string(asset),
			}
			if addr != from {
				// legacy endpoint drops 'change' movements
				destinations = append(destinations, endpoint)
			} else {
				txWithInfo.AddDroppedDestination(endpoint)
			}
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
	chain := client.Asset.GetChain().Chain

	// delete the fee to avoid double counting.
	// the new model will calculate fees from the difference of inflows/outflows
	legacyTx.Fee = xc.NewAmountBlockchainFromUint64(0)

	// add back the change movements
	legacyTx.Destinations = append(legacyTx.Destinations, legacyTx.GetDroppedBtcDestinations()...)

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Utxo), nil
}

func (client *BlockbookClient) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
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

func (client *BlockbookClient) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalance(ctx, address)
}

func (client *BlockbookClient) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	input := tx_input.NewTxInput()
	allUnspentOutputs, err := client.UnspentOutputs(ctx, xc.Address(from))
	if err != nil {
		return input, err
	}
	input.UnspentOutputs = allUnspentOutputs
	gasPerByte, err := client.EstimateFee(ctx)
	input.GasPricePerByte = gasPerByte
	if err != nil {
		return input, err
	}

	return input, nil
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

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		var errResponse ErrorResponse
		err = json.Unmarshal(body, &errResponse)
		if err == nil {
			return fmt.Errorf("failed to get %s: %s", path, errResponse.Error)
		}
		return fmt.Errorf("failed to get %s: code=%d", path, res.StatusCode)
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
	url := fmt.Sprintf("%s/%s", client.Url, path)
	logrus.WithFields(logrus.Fields{
		"url":  url,
		"body": string(input),
	}).Debug("post")
	res, err := client.httpClient.Post(url, contentType, bytes.NewReader(input))
	if err != nil {
		return fmt.Errorf("blockbook post failed: %v", err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		var errResponse ErrorResponse
		err = json.Unmarshal(body, &errResponse)
		if err == nil {
			return fmt.Errorf("failed to get %s: %s", path, errResponse.Error)
		}
		return fmt.Errorf("failed to get %s: code=%d", path, res.StatusCode)
	}

	if resp != nil {
		err = json.Unmarshal(body, &resp)
		if err != nil {
			return err
		}
	}
	return nil
}
