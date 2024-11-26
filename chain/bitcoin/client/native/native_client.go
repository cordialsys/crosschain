package native

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/shopspring/decimal"
)

var (
	// default timeout for client
	DefaultClientTimeout = time.Minute
	// default retry period for failed connections
	DefaultClientTimeoutRetry = 10 * time.Second
	// default host to connect to rpc node
	DefaultClientHost = "http://0.0.0.0:18443"
	// default user for rpc connection
	DefaultClientUser = "user"
	// default password for rpc connection
	DefaultClientPassword = "password"
	// default auth header for rpc connection
	DefaultClientAuthHeader = ""
)

func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		Timeout:      DefaultClientTimeout,
		TimeoutRetry: DefaultClientTimeoutRetry,
		Host:         DefaultClientHost,
		User:         DefaultClientUser,
		Password:     DefaultClientPassword,
		AuthHeader:   DefaultClientAuthHeader,
	}
}

// ClientOptions are used to parameterise the behaviour of the Client.
type ClientOptions struct {
	Timeout         time.Duration
	TimeoutRetry    time.Duration
	Host            string
	User            string
	Password        string
	AuthHeader      string
	AuthHeaderValue string
	Chaincfg        *chaincfg.Params
}

// Client for Bitcoin
type NativeClient struct {
	opts           ClientOptions
	httpClient     http.Client
	Asset          xc.ITask
	addressDecoder address.AddressDecoder
}

var _ xclient.Client = &NativeClient{}
var _ xclient.ClientWithDecimals = &NativeClient{}
var _ address.WithAddressDecoder = &NativeClient{}

// NewClient returns a new Bitcoin Client
func NewNativeClient(cfgI xc.ITask) (*NativeClient, error) {
	native := cfgI.GetChain()
	opts := DefaultClientOptions()
	httpClient := http.Client{}
	httpClient.Timeout = opts.Timeout
	opts.Host = native.URL
	params, err := params.GetParams(native)
	if err != nil {
		return &NativeClient{}, err
	}
	opts.Chaincfg = params
	return &NativeClient{
		opts:       opts,
		httpClient: httpClient,
		Asset:      cfgI,
	}, nil
}

func (txBuilder *NativeClient) WithAddressDecoder(decoder address.AddressDecoder) address.WithAddressDecoder {
	txBuilder.addressDecoder = decoder
	return txBuilder
}

// FetchTransferInput returns tx input for a Bitcoin tx
func (client *NativeClient) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input := tx_input.NewTxInput()
	allUnspentOutputs, err := client.UnspentOutputs(ctx, 0, 999999999, args.GetFrom())
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

// SubmitTx submits a Bitcoin tx
func (client *NativeClient) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	serial, err := txInput.Serialize()
	if err != nil {
		return fmt.Errorf("bad tx: %v", err)
	}
	resp := ""
	if err := client.send(ctx, &resp, "sendrawtransaction", hex.EncodeToString(serial)); err != nil {
		return fmt.Errorf("bad \"sendrawtransaction\": %v", err)
	}
	return nil
}
func (client *NativeClient) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// FetchLegacyTxInfo returns tx info for a Bitcoin tx
func (client *NativeClient) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	resp := btcjson.GetTransactionResult{}

	// TODO use to-address to figure out the current amount in btc transfer.
	expectedTo := ""

	if err := client.send(ctx, &resp, "gettransaction", txHash); err != nil {
		return xc.LegacyTxInfo{}, fmt.Errorf("bad \"gettransaction\": %v", err)
	}
	j1, _ := json.Marshal(resp)
	log.Printf("res: %s", j1)

	// need to use decimal to avoid rounding issues with floats
	feeDec := decimal.NewFromFloat(resp.Fee)
	feeDec = feeDec.Abs().Shift(8)
	fee := feeDec.BigInt()

	sources := []*xc.LegacyTxInfoEndpoint{}
	destinations := []*xc.LegacyTxInfoEndpoint{}

	data, _ := hex.DecodeString(resp.Hex)
	txObject := &tx.Tx{
		Input:  tx_input.NewTxInput(),
		MsgTx:  &wire.MsgTx{},
		Signed: true,
	}
	txObject.MsgTx.Deserialize(bytes.NewReader(data))
	inputs := []tx.Input{}
	// btc chains the native asset and asset are the same
	asset := client.Asset.GetChain().Chain

	// extract tx.inputs
	// this is just raw data from the blockchain
	for _, txIn := range txObject.MsgTx.TxIn {
		hash := make([]byte, len(txIn.PreviousOutPoint.Hash))
		copy(hash[:], txIn.PreviousOutPoint.Hash[:])

		outpoint := tx_input.Outpoint{
			Hash:  hash,
			Index: txIn.PreviousOutPoint.Index,
		}
		output, _, err := client.Output(ctx, outpoint)
		if err != nil {
			return xc.LegacyTxInfo{}, fmt.Errorf("error retrieving input details: %v", err)
		}
		_, addresses, _, err := txscript.ExtractPkScriptAddrs(output.PubKeyScript, client.opts.Chaincfg)
		if err != nil || len(addresses) != 1 {
			return xc.LegacyTxInfo{}, fmt.Errorf("error extracting address from input: %v", err)
		}
		input := tx.Input{
			Output:  output,
			Address: xc.Address(addresses[0].String()),
		}
		inputs = append(inputs, input)
		txObject.Input.UnspentOutputs = append(txObject.Input.UnspentOutputs, input.Output)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address:         input.Address,
			Amount:          input.Value,
			ContractAddress: "",
			NativeAsset:     xc.NativeAsset(asset),
			Asset:           string(asset),
		})
	}

	// detect from address
	// single input: from is the address of the single input = unspent output
	// multiple inputs: from is the address of the unspent output with highest value
	from, totalIn := tx.DetectFrom(inputs)

	// detect recipient addresses and fields: to, amount
	// two outputs:
	// - to is the address which is not the sender
	// - amount is the value received
	// more outputs: not really well defined, currently the last recipient
	outputs, _ := txObject.Outputs()
	for _, output := range outputs {
		value := output.Value
		_, addresses, _, err := txscript.ExtractPkScriptAddrs(output.PubKeyScript, client.opts.Chaincfg)
		if err != nil || len(addresses) != 1 {
			return xc.LegacyTxInfo{}, fmt.Errorf("error extracting address from output: %v", err)
		}
		recipientAddr := addresses[0].String()
		recipient := tx.Recipient{
			To:    xc.Address(recipientAddr),
			Value: value,
		}
		txObject.Recipients = append(txObject.Recipients, recipient)
		destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(recipientAddr),
			ContractAddress: "",
			Amount:          value,
			NativeAsset:     xc.NativeAsset(asset),
			Asset:           string(asset),
		})
	}

	to, amount, totalOut := txObject.DetectToAndAmount(from, expectedTo)
	if resp.Fee == 0 && totalIn.Cmp(&totalOut) > 0 {
		newfee := totalIn.Sub(&totalOut)
		fee = (*big.Int)(&newfee)
	}

	return xc.LegacyTxInfo{
		From:          xc.Address(from),
		To:            xc.Address(to),
		Amount:        amount,
		Fee:           *((*xc.AmountBlockchain)(fee)),
		Confirmations: resp.Confirmations,
		BlockHash:     resp.BlockHash,
		BlockIndex:    resp.BlockIndex,
		BlockTime:     resp.BlockTime,
		Sources:       sources,
		Destinations:  destinations,
		TxID:          resp.TxID,
		Time:          resp.Time,
		TimeReceived:  resp.TimeReceived,
	}, nil
}

func (client *NativeClient) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain()
	// delete the fee to avoid double counting.
	// the new model will calculate fees from the difference of inflows/outflows
	legacyTx.Fee = xc.NewAmountBlockchainFromUint64(0)

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Utxo), nil
}

func (client *NativeClient) send(ctx context.Context, resp interface{}, method string, params ...interface{}) error {
	// Encode the request.
	data, err := encodeRequest(method, params)
	if err != nil {
		return err
	}

	return retry(ctx, client.opts.TimeoutRetry, func() error {
		// Create request and add basic authentication headers. The context is
		// not attached to the request, and instead we all each attempt to run
		// for the timeout duration, and we keep attempting until success, or
		// the context is done.
		req, err := http.NewRequest("POST", client.opts.Host, bytes.NewBuffer(data))
		if err != nil {
			return fmt.Errorf("building http request: %v", err)
		}
		req.SetBasicAuth(client.opts.User, client.opts.Password)
		if client.opts.AuthHeader != "" {
			req.Header.Set(client.opts.AuthHeader, client.opts.AuthHeaderValue)
		}

		// Send the request and decode the response.
		res, err := client.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("sending http request: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode == 401 {
			return fmt.Errorf("http response: %v", res.Status)
		}
		if err := decodeResponse(resp, res.Body); err != nil {
			return fmt.Errorf("decoding http response: %v", err)
		}
		return nil
	})
}

// UnspentOutputs spendable by the given address.
func (client *NativeClient) UnspentOutputs(ctx context.Context, minConf, maxConf int64, addr xc.Address) ([]tx_input.Output, error) {
	resp := []btcjson.ListUnspentResult{}
	if err := client.send(ctx, &resp, "listunspent", minConf, maxConf, []string{string(addr)}); err != nil && err != io.EOF {
		return []tx_input.Output{}, fmt.Errorf("bad \"listunspent\": %v", err)
	}
	outputs := make([]tx_input.Output, len(resp))
	for i := range outputs {
		amount, err := btcutil.NewAmount(resp[i].Amount)
		if err != nil {
			return []tx_input.Output{}, fmt.Errorf("bad amount: %v", err)
		}
		if amount < 0 {
			return []tx_input.Output{}, fmt.Errorf("bad amount: %v", amount)
		}
		pubKeyScript, err := hex.DecodeString(resp[i].ScriptPubKey)
		if err != nil {
			return []tx_input.Output{}, fmt.Errorf("bad pubkey script: %v", err)
		}
		txid, err := chainhash.NewHashFromStr(resp[i].TxID)
		if err != nil {
			return []tx_input.Output{}, fmt.Errorf("bad txid: %v", err)
		}
		outputs[i] = tx_input.Output{
			Outpoint: tx_input.Outpoint{
				Hash:  txid[:],
				Index: resp[i].Vout,
			},
			Value:        xc.NewAmountBlockchainFromUint64(uint64(amount)),
			PubKeyScript: pubKeyScript,
		}
	}
	return outputs, nil
}

func (client *NativeClient) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
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
func (client *NativeClient) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalance(ctx, address)
}

// Older version of estimating fee for some forks of BTC (e.g. BCH).
func (client *NativeClient) EstimateFeeLegacy(ctx context.Context, numBlocks int64) (float64, error) {
	var resp float64

	switch numBlocks {
	case int64(0):
		if err := client.send(ctx, &resp, "estimatefee"); err != nil {
			return 0.0, fmt.Errorf("estimating fee: %v", err)
		}
	default:
		if err := client.send(ctx, &resp, "estimatefee", numBlocks); err != nil {
			return 0.0, fmt.Errorf("estimating fee: %v", err)
		}
	}

	return resp, nil
}

// Latest way to estimate fees on BTC
func (client *NativeClient) EstimateSmartFee(ctx context.Context, numBlocks int64) (float64, error) {
	resp := btcjson.EstimateSmartFeeResult{}

	if err := client.send(ctx, &resp, "estimatesmartfee", numBlocks); err != nil {
		return 0.0, fmt.Errorf("estimating smart fee: %v", err)
	}

	if resp.Errors != nil && len(resp.Errors) > 0 {
		return 0.0, fmt.Errorf("estimating smart fee: %v", resp.Errors[0])
	}

	return *resp.FeeRate, nil
}

// Import an address into the RPC node to be tracked.
// func (client *NativeClient) ImportAddress(ctx context.Context, addr address.Address, label string, rescan bool) error {
// 	if err := client.send(ctx, nil, "importaddress", string(addr), label, rescan); err != nil {
// 		return fmt.Errorf("import address: %v", err)
// 	}
// 	return nil
// }

func (client *NativeClient) GetWalletInfo(ctx context.Context) (float64, error) {
	resp := btcjson.GetWalletInfoResult{}
	if err := client.send(ctx, &resp, "getwalletinfo"); err != nil {
		return 0, fmt.Errorf("import address: %v", err)
	}
	j, _ := json.Marshal(resp)
	log.Printf("getwalletinfo: %s", j)
	if resp.Scanning.Value != nil {
		switch v := resp.Scanning.Value; v.(type) {
		case btcjson.ScanProgress:
			return v.(btcjson.ScanProgress).Progress, nil
		}
	}
	return 0, nil
}

// LatestBlock returns the height of the longest blockchain.
func (client *NativeClient) LatestBlock(ctx context.Context) (uint64, error) {
	var resp int64
	if err := client.send(ctx, &resp, "getblockcount"); err != nil {
		return 0, fmt.Errorf("get block count: %v", err)
	}
	if resp < 0 {
		return 0, fmt.Errorf("unexpected block count, expected > 0, got: %v", resp)
	}

	return uint64(resp), nil
}

// Output associated with an outpoint, and its number of confirmations.
func (client *NativeClient) Output(ctx context.Context, outpoint tx_input.Outpoint) (tx_input.Output, uint64, error) {
	resp := btcjson.TxRawResult{}
	hash := chainhash.Hash{}
	copy(hash[:], outpoint.Hash)
	if err := client.send(ctx, &resp, "getrawtransaction", hash.String(), 1); err != nil {
		return tx_input.Output{}, 0, fmt.Errorf("bad \"getrawtransaction\": %v", err)
	}
	if outpoint.Index >= uint32(len(resp.Vout)) {
		return tx_input.Output{}, 0, fmt.Errorf("bad index: %v is out of range", outpoint.Index)
	}
	vout := resp.Vout[outpoint.Index]
	amount, err := btcutil.NewAmount(vout.Value)
	if err != nil {
		return tx_input.Output{}, 0, fmt.Errorf("bad amount: %v", err)
	}
	if amount < 0 {
		return tx_input.Output{}, 0, fmt.Errorf("bad amount: %v", amount)
	}
	pubKeyScript, err := hex.DecodeString(vout.ScriptPubKey.Hex)
	if err != nil {
		return tx_input.Output{}, 0, fmt.Errorf("bad pubkey script: %v", err)
	}
	output := tx_input.Output{
		Outpoint:     outpoint,
		Value:        xc.NewAmountBlockchainFromUint64(uint64(amount)),
		PubKeyScript: pubKeyScript,
	}
	return output, resp.Confirmations, nil
}

func (client *NativeClient) EstimateGas(ctx context.Context) (xc.AmountBlockchain, error) {
	// estimate using last 1 blocks
	numBlocks := 1
	fallbackGasPerByte := xc.NewAmountBlockchainFromUint64(2)
	satsPerByteFloat, err := client.EstimateSmartFee(ctx, int64(numBlocks))
	if err != nil {
		return fallbackGasPerByte, err
	}

	if satsPerByteFloat <= 0.0 {
		return fallbackGasPerByte, fmt.Errorf("invalid sats per byte: %v", satsPerByteFloat)
	}

	satsPerByte := uint64(satsPerByteFloat)

	return xc.NewAmountBlockchainFromUint64(satsPerByte), nil
}

func (client *NativeClient) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	return 0, fmt.Errorf("unsupported")
}
