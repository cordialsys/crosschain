package substrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/api"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// Client for Substrate
type Client struct {
	DotClient    *gsrpc.SubstrateAPI
	Asset        xc.ITask
	substrateUrl string
	apiKey       string
}

var _ xclient.FullClient = &Client{}

// TxInput for Substrate
type TxInput struct {
	xc.TxInputEnvelope
	Meta          Metadata             `json:"meta,omitempty"`
	GenesisHash   types.Hash           `json:"genesis_hash,omitempty"`
	CurHash       types.Hash           `json:"current_hash,omitempty"`
	Rv            types.RuntimeVersion `json:"runtime_version,omitempty"`
	CurrentHeight uint64               `json:"current_height,omitempty"`
	Tip           uint64               `json:"tip,omitempty"`
	Nonce         uint64               `json:"account_nonce,omitempty"`
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	multipliedTip := multiplier.Mul(decimal.NewFromInt(int64(input.Tip)))
	input.Tip = multipliedTip.BigInt().Uint64()
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if substrateOther, ok := other.(*TxInput); ok {
		return substrateOther.Nonce != input.Nonce
	}
	return
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	// all same sequence means no double send
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// sequence all same - we're safe
	return true
}

// NewClient returns a new Substrate Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	rpcurl := cfgI.GetChain().URL

	txInfoClientI, err := NewTxInfoClient(cfgI)
	if err != nil {
		return nil, err
	}
	txInfoClient := txInfoClientI.(*Client)

	client, err := gsrpc.NewSubstrateAPI(rpcurl)
	return &Client{
		DotClient:    client,
		Asset:        cfgI,
		substrateUrl: txInfoClient.substrateUrl,
		apiKey:       txInfoClient.apiKey,
	}, err
}

type TxInfoClient interface {
	// Fetching transaction info - legacy endpoint
	FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error)
	// Fetching transaction info
	FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error)
}

func NewTxInfoClient(cfgI xc.ITask) (TxInfoClient, error) {
	indexerUrl := cfgI.GetChain().IndexerUrl
	apiKey := cfgI.GetChain().AuthSecret

	help := `The substrate driver relies on a supported subscan endpoint (https://support.subscan.io).\n` +
		`This is used only to download transactions (extrinics) by their hash, as this is not natively supported by substrate chains.`
	if indexerUrl == "" {
		return nil, fmt.Errorf(`must set .indexer_url\n` + help)
	}
	if apiKey == "" {
		return nil, fmt.Errorf(`must set .api-key\n` + help)
	}
	indexerUrl = strings.TrimSuffix(indexerUrl, "/")
	return &Client{
		Asset:        cfgI,
		substrateUrl: indexerUrl,
		apiKey:       apiKey,
	}, nil
}

// NewTxInput returns a new Substrate TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSubstrate),
	}
}

func (client *Client) FetchTxInputChain() (*types.Metadata, *TxInput, error) {
	txInput := NewTxInput()
	rpc := client.DotClient.RPC
	meta, err := rpc.State.GetMetadataLatest()
	if err != nil {
		return meta, &TxInput{}, err
	}
	txInput.Meta, err = ParseMeta(meta)
	if err != nil {
		return meta, &TxInput{}, err
	}
	// txInput.MetaData2 = *meta
	txInput.GenesisHash, err = rpc.Chain.GetBlockHash(0)
	if err != nil {
		return meta, &TxInput{}, err
	}
	rv, err := rpc.State.GetRuntimeVersionLatest()
	if err != nil {
		return meta, &TxInput{}, err
	}
	txInput.Rv = *rv
	header, err := rpc.Chain.GetHeaderLatest()
	if err != nil {
		return meta, &TxInput{}, err
	}
	txInput.CurrentHeight = uint64(header.Number)
	txInput.CurHash, err = rpc.Chain.GetBlockHash(txInput.CurrentHeight)
	if err != nil {
		return meta, &TxInput{}, err
	}
	return meta, txInput, nil
}

func (client *Client) FetchAccountNonce(meta types.Metadata, from xc.Address) (uint64, error) {
	sender, err := types.NewMultiAddressFromAccountID(base58.Decode(string(from))[1:33])
	if err != nil {
		return 0, err
	}
	storageKey, err := types.CreateStorageKey(&meta, "System", "Account", sender.AsID[:])
	if err != nil {
		return 0, err
	}
	var accountInfo api.AccountInfoMinimal
	ok, err := client.DotClient.RPC.State.GetStorageLatest(storageKey, &accountInfo)
	if err != nil || !ok {
		return 0, err
	}
	return uint64(accountInfo.Nonce), nil
}

// FetchTxInput returns tx input for a Substrate tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	meta, txInput, err := client.FetchTxInputChain()
	if err != nil {
		return &TxInput{}, err
	}
	txInput.Nonce, err = client.FetchAccountNonce(*meta, from)
	if err != nil {
		return &TxInput{}, err
	}
	amt, err := client.EstimateTip(ctx)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chain": client.Asset.GetChain().Chain,
			"error": err,
		}).Warn("could not estimate gas fee")
	}
	txInput.Tip = amt

	return txInput, nil
}

type RpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// The current rpc client omits the .data in it's err.Error() method
func AsRpcErrorMaybe(inputError error) error {
	bz, err := json.Marshal(inputError)
	if err != nil {
		return inputError
	}
	var outputError RpcError
	err = json.Unmarshal(bz, &outputError)
	if err != nil {
		return inputError
	}
	if outputError.Code != 0 && len(outputError.Message) > 0 {
		if outputError.Data != nil {
			return fmt.Errorf("%s: %v (%d)", outputError.Message, outputError.Data, outputError.Code)
		} else {
			return fmt.Errorf("%s (%d)", outputError.Message, outputError.Code)
		}
	}
	return inputError
}

// SubmitTx submits a Substrate tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	data, err := txInput.Serialize()
	if err != nil {
		return err
	}

	var res string
	encoded := codec.HexEncodeToString(data)
	logrus.WithField("tx", encoded).Debug("submitting tx")
	err = client.DotClient.Client.Call(&res, "author_submitExtrinsic", encoded)
	if err != nil {
		return AsRpcErrorMaybe(err)
	}
	return nil
}

func (client *Client) ParseTxInfo(body []byte) (xc.LegacyTxInfo, error) {
	var txInfoResp api.SubscanExtrinsicResponse
	err := json.Unmarshal(body, &txInfoResp)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	if len(txInfoResp.Data.BlockHash) == 0 {
		return xc.LegacyTxInfo{}, errors.New("not found")
	}

	sources := []*xc.LegacyTxInfoEndpoint{}
	destinations := []*xc.LegacyTxInfoEndpoint{}
	addressBuilder, err := NewAddressBuilder(client.Asset)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	chain := client.Asset.GetChain().Chain

	// parse known events
	for _, ev := range txInfoResp.Data.Event {
		handle := ev.ModuleID + "." + ev.EventID
		switch handle {
		case "balances.Transfer":
			_, err := ev.ParseParams()
			if err != nil {
				return xc.LegacyTxInfo{}, err
			}
			fromId, err := api.GetParamAccountId(&ev, "from")
			if err != nil {
				return xc.LegacyTxInfo{}, err
			}
			toId, err := api.GetParamAccountId(&ev, "to")
			if err != nil {
				return xc.LegacyTxInfo{}, err
			}
			amount, err := api.GetParamInt(&ev, "amount")
			if err != nil {
				return xc.LegacyTxInfo{}, err
			}

			from, _ := addressBuilder.GetAddressFromPublicKey(fromId)
			to, _ := addressBuilder.GetAddressFromPublicKey(toId)

			sources = append(sources, &xc.LegacyTxInfoEndpoint{
				Address:     from,
				NativeAsset: chain,
				Amount:      amount,
			})
			destinations = append(destinations, &xc.LegacyTxInfoEndpoint{
				Address:     to,
				NativeAsset: chain,
				Amount:      amount,
			})
		}

	}
	amount := xc.NewAmountBlockchainFromUint64(0)
	from := xc.Address("")
	to := xc.Address("")
	if len(sources) > 0 {
		from = sources[0].Address
	}
	if len(destinations) > 0 {
		amount = destinations[0].Amount
		to = destinations[0].Address
	}

	return xc.LegacyTxInfo{
		BlockHash:    txInfoResp.Data.BlockHash,
		TxID:         txInfoResp.Data.ExtrinsicHash,
		From:         from,
		To:           to,
		Amount:       amount,
		Fee:          xc.NewAmountBlockchainFromStr(txInfoResp.Data.Fee),
		BlockIndex:   int64(txInfoResp.Data.BlockNum),
		BlockTime:    int64(txInfoResp.Data.BlockTimestamp),
		Sources:      sources,
		Destinations: destinations,
	}, nil
}

// FetchLegacyTxInfo returns tx info for a Substrate tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	if !strings.HasPrefix(string(txHash), "0x") {
		txHash = "0x" + txHash
	}
	var reqBody = []byte(`{"hash": "` + txHash + `"}`)

	req, err := http.NewRequest("POST", client.substrateUrl+"/api/scan/extrinsic", bytes.NewBuffer(reqBody))
	req.Header.Add("Content-Type", "application/json")
	if client.apiKey != "" {
		req.Header.Add("X-API-Key", client.apiKey)
	}
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	explorerClient := &http.Client{}
	resp, err := explorerClient.Do(req)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	tx, err := client.ParseTxInfo(body)
	if err != nil {
		return tx, err
	}
	if client.DotClient != nil {
		// calculate confirmations
		header, err := client.DotClient.RPC.Chain.GetHeaderLatest()
		if err != nil {
			return tx, err
		}
		tx.Confirmations = int64(header.Number) - tx.BlockIndex
	}
	return tx, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain().Chain, legacyTx, xclient.Account), nil
}

// FetchNativeBalance fetches account balance for a Substrate address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	meta, err := client.DotClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return zero, err
	}

	key, err := types.CreateStorageKey(meta, "System", "Account", base58.Decode(string(address))[1:33])
	if err != nil {
		return zero, err
	}

	var acctInfo api.AccountInfoMinimal
	// var acctInfo types.AccountInfo
	ok, err := client.DotClient.RPC.State.GetStorageLatest(key, &acctInfo)
	if err != nil || !ok {
		return zero, err
	}

	return xc.NewAmountBlockchainFromUint64(acctInfo.Data.Free.Uint64()), nil
}

// FetchBalance fetches token balance for a Substrate address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	if client.Asset.GetContract() == "" {
		return client.FetchNativeBalance(ctx, address)
	} else {
		return xc.AmountBlockchain{}, errors.New("unsupported asset")
	}
}

// EstimateTip looks at the latest extrinsics to try to calculate an average tip paid
func (client *Client) EstimateTip(ctx context.Context) (uint64, error) {
	block, err := client.DotClient.RPC.Chain.GetBlockLatest()
	if err != nil {
		return 0, err
	}

	var total uint64
	var count uint64
	for _, ext := range block.Block.Extrinsics {
		tip := ext.Signature.Tip.Int64()
		if tip > 0 {
			total += uint64(tip)
			count += 1
		}
	}
	if count < 5 {
		return 0, nil
	}

	return total / count, nil
}
