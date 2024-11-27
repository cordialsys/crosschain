package substrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/substrate/api"
	"github.com/cordialsys/crosschain/chain/substrate/api/graphql"
	"github.com/cordialsys/crosschain/chain/substrate/api/subscan"
	"github.com/cordialsys/crosschain/chain/substrate/api/taostats"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// Client for Substrate
type Client struct {
	DotClient  *gsrpc.SubstrateAPI
	Asset      xc.ITask
	indexerUrl string
	apiKey     string
}

const IndexerSubQuery = "subquery"
const IndexerSubScan = "subscan"
const IndexerTaostats = "taostats"

var SupportedIndexers = []string{IndexerSubQuery, IndexerSubScan, IndexerTaostats}

var _ xclient.FullClient = &Client{}
var _ xclient.ClientWithDecimals = &Client{}

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

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverSubstrate
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
	if err != nil {
		// We sack error here since we don't want to fail on connectivity in contructor.
		// instead, we'll fail later when FetchBalance or something is called.
		logrus.Warnf("invalid rpc url: %v", err)
	}
	return &Client{
		DotClient:  client,
		Asset:      cfgI,
		indexerUrl: txInfoClient.indexerUrl,
		apiKey:     txInfoClient.apiKey,
	}, nil
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

	help := fmt.Sprintf(`The substrate driver relies on a supported subscan indexer (%v).\n`+
		`This is used only to download transactions (extrinics) by their hash, as this is not natively supported by substrate chains.`, SupportedIndexers)
	if indexerUrl == "" {
		return nil, fmt.Errorf(`must set .indexer_url\n` + help)
	}
	if cfgI.GetChain().IndexerType == IndexerSubQuery {
		// do not require api key
	} else {
		if apiKey == "" {
			return nil, fmt.Errorf(`must set .api-key\n` + help)
		}
	}
	indexerUrl = strings.TrimSuffix(indexerUrl, "/")
	return &Client{
		Asset:      cfgI,
		indexerUrl: indexerUrl,
		apiKey:     apiKey,
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

// FetchLegacyTxInput returns tx input for a Substrate tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	meta, txInput, err := client.FetchTxInputChain()
	if err != nil {
		return &TxInput{}, err
	}
	txInput.Nonce, err = client.FetchAccountNonce(*meta, args.GetFrom())
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
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
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

// FetchLegacyTxInfo returns tx info for a Substrate tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	var tx xc.LegacyTxInfo

	addressBuilder, err := NewAddressBuilder(client.Asset)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}
	chain := client.Asset.GetChain().Chain
	var eventsI = []api.EventI{}

	if client.Asset.GetChain().IndexerType == IndexerSubQuery {
		extrinsicQuery := fmt.Sprintf(
			`{"query":"query { extrinsics(first: 1, offset: 0, filter: { or: [{txHash: {equalTo:\"%s\"}}, { id: {equalTo:\"%s\"} }]} , orderBy: ID_DESC) {nodes {id txHash tip}} }"}`,
			txHash, txHash,
		)
		var response graphql.SubqueryExtrinsicResponse
		err := graphql.Post(ctx, client.indexerUrl, []byte(extrinsicQuery), &response, &graphql.ClientArgs{ApiKey: client.apiKey})
		if err != nil {
			return xc.LegacyTxInfo{}, err
		}

		if len(response.Data.Extrinsics.Nodes) == 0 {
			return xc.LegacyTxInfo{}, fmt.Errorf("no transaction found by hash %s", txHash)
		}
		ext := response.Data.Extrinsics.Nodes[0]
		height, offset, err := ext.ID.Parse()
		if err != nil {
			return xc.LegacyTxInfo{}, err
		}
		eventsQuery := fmt.Sprintf(
			`{"query":"query {      events(first: 100, offset: 0, filter: {blockHeight:{equalTo:\"%d\"} extrinsicId:{equalTo: %d}}, orderBy: ID_DESC) { nodes { module event data } } blocks(first: 1, offset: 0, filter: {height:{equalTo:\"%d\"} }, orderBy: ID_DESC) { nodes { timestamp hash } } } "}`,
			height, offset, height,
		)
		var eventsResponse graphql.SubqueryEventResponse
		err = graphql.Post(ctx, client.indexerUrl, []byte(eventsQuery), &eventsResponse, &graphql.ClientArgs{ApiKey: client.apiKey})
		if err != nil {
			return xc.LegacyTxInfo{}, err
		}
		if len(eventsResponse.Data.Blocks.Nodes) == 0 {
			return xc.LegacyTxInfo{}, fmt.Errorf("no block found at height %d", height)
		}
		block := eventsResponse.Data.Blocks.Nodes[0]
		for _, ev := range eventsResponse.Data.Events.Nodes {
			_, err := ev.ParseParams()
			if err != nil {
				return xc.LegacyTxInfo{}, err
			}
			eventsI = append(eventsI, ev)
		}
		tx.BlockHash = block.Hash
		tx.TxID = ext.TxHash
		tx.Fee = xc.NewAmountBlockchainFromStr(ext.Tip)
		tx.BlockIndex = int64(height)
		tx.BlockTime = block.Timestamp.Unix()
	} else if client.Asset.GetChain().IndexerType == IndexerTaostats {
		taostatClient := taostats.NewClient(client.indexerUrl, client.apiKey)
		ext, err := taostatClient.GetTransaction(ctx, string(txHash))
		if err != nil {
			return xc.LegacyTxInfo{}, err
		}

		block, err := taostatClient.GetBlock(ctx, ext.BlockNumber)
		if err != nil {
			return xc.LegacyTxInfo{}, err
		}

		events, err := taostatClient.GetEvents(ctx, ext)
		if err != nil {
			return xc.LegacyTxInfo{}, err
		}

		for _, event := range events {
			eventsI = append(eventsI, event)
		}
		tx.BlockHash = block.Hash
		tx.TxID = ext.Hash
		tx.Fee = xc.NewAmountBlockchainFromStr(ext.Fee)
		tx.BlockIndex = ext.BlockNumber
		tx.BlockTime = ext.Timestamp.Unix()

	} else {
		// support querying by either hash and extrinsic ID
		var reqBody string
		if _, _, err = api.BlockAndOffset(txHash).Parse(); err == nil {
			reqBody = `{"extrinsic_index": "` + string(txHash) + `"}`
		} else {
			if !strings.HasPrefix(string(txHash), "0x") {
				txHash = "0x" + txHash
			}
			reqBody = `{"hash": "` + string(txHash) + `"}`
		}

		fmt.Println(txHash, string(reqBody))
		var txInfoResp subscan.SubscanExtrinsicResponse
		subscan.Post(ctx, client.indexerUrl+"/api/scan/extrinsic", []byte(reqBody), &txInfoResp, &subscan.ClientArgs{ApiKey: client.apiKey})
		if len(txInfoResp.Data.BlockHash) == 0 {
			return xc.LegacyTxInfo{}, errors.New("not found")
		}

		for _, ev := range txInfoResp.Data.Event {
			_, err := ev.ParseParams()
			if err != nil {
				return xc.LegacyTxInfo{}, err
			}
			eventsI = append(eventsI, ev)
		}
		tx.BlockHash = txInfoResp.Data.BlockHash
		tx.TxID = txInfoResp.Data.ExtrinsicHash
		tx.Fee = xc.NewAmountBlockchainFromStr(txInfoResp.Data.Fee)
		tx.BlockIndex = int64(txInfoResp.Data.BlockNum)
		tx.BlockTime = int64(txInfoResp.Data.BlockTimestamp)
	}
	if client.DotClient != nil && tx.Confirmations == 0 {
		// calculate confirmations
		header, err := client.DotClient.RPC.Chain.GetHeaderLatest()
		if err != nil {
			return tx, err
		}
		tx.Confirmations = int64(header.Number) - tx.BlockIndex
	}

	tx.Sources, tx.Destinations, err = api.ParseEvents(addressBuilder, chain, eventsI)
	if err != nil {
		return xc.LegacyTxInfo{}, err
	}

	if len(tx.Sources) > 0 {
		tx.From = tx.Sources[0].Address
	}
	if len(tx.Destinations) > 0 {
		tx.Amount = tx.Destinations[0].Amount
		tx.To = tx.Destinations[0].Address
	}

	return tx, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain(), legacyTx, xclient.Account), nil
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
	contract := client.Asset.GetContract()
	if contract == "" {
		return client.FetchNativeBalance(ctx, address)
	} else {
		return xc.AmountBlockchain{}, fmt.Errorf("unsupported asset: %v", contract)
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

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	return 0, fmt.Errorf("unsupported asset: %v", contract)
}
