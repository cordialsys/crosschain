package substrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/jumpcrypto/crosschain"
)

// Client for Substrate
type Client struct {
	DotClient       *gsrpc.SubstrateAPI
	Asset           xc.ITask
	EstimateGasFunc xc.EstimateGasFunc
}

var _ xc.Client = &Client{}

// TxInput for Substrate
type TxInput struct {
	xc.TxInputEnvelope
	Meta        types.Metadata       `json:"meta,omitempty"`
	GenesisHash types.Hash           `json:"genesis_hash,omitempty"`
	CurHash     types.Hash           `json:"current_hash,omitempty"`
	Rv          types.RuntimeVersion `json:"runtime_version,omitempty"`
	CurNum      uint64               `json:"current_num,omitempty"`
	Tip         uint64               `json:"tip,omitempty"`
	Nonce       uint64               `json:"nonce,omitempty"`
}

// NewClient returns a new Substrate Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	rpcurl := cfgI.GetNativeAsset().URL
	if rpcurl != "" {
		client, err := gsrpc.NewSubstrateAPI(rpcurl)
		return &Client{
			DotClient: client,
			Asset:     cfgI,
		}, err
	} else {
		// Gracefully continue even if no URL provided (still include asset in returned client)
		return &Client{
			Asset: cfgI,
		}, errors.New("bad rpc url")
	}
}

// NewTxInput returns a new Substrate TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSubstrate),
	}
}

func (client *Client) FetchTxInputChain() (*TxInput, error) {
	txInput := NewTxInput()
	rpc := client.DotClient.RPC
	meta, err := rpc.State.GetMetadataLatest()
	if err != nil {
		return &TxInput{}, err
	}
	txInput.Meta = *meta
	txInput.GenesisHash, err = rpc.Chain.GetBlockHash(0)
	if err != nil {
		return &TxInput{}, err
	}
	rv, err := rpc.State.GetRuntimeVersionLatest()
	if err != nil {
		return &TxInput{}, err
	}
	txInput.Rv = *rv
	header, err := rpc.Chain.GetHeaderLatest()
	if err != nil {
		return &TxInput{}, err
	}
	txInput.CurNum = uint64(header.Number)
	txInput.CurHash, err = rpc.Chain.GetBlockHash(txInput.CurNum)
	if err != nil {
		return &TxInput{}, err
	}
	return txInput, nil
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
	var accountInfo types.AccountInfo
	ok, err := client.DotClient.RPC.State.GetStorageLatest(storageKey, &accountInfo)
	if err != nil || !ok {
		return 0, err
	}
	return uint64(accountInfo.Nonce), nil
}

// FetchTxInput returns tx input for a Substrate tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	txInput, err := client.FetchTxInputChain()
	if err != nil {
		return &TxInput{}, err
	}
	txInput.Tip = client.Asset.GetAssetConfig().ChainGasTip
	txInput.Nonce, err = client.FetchAccountNonce(txInput.Meta, from)
	if err != nil {
		return &TxInput{}, err
	}
	return txInput, nil
}

// SubmitTx submits a Substrate tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	data, err := txInput.Serialize()
	if err != nil {
		return err
	}

	var res string
	err = client.DotClient.Client.Call(&res, "author_submitExtrinsic", codec.HexEncodeToString(data))
	if err != nil {
		return err
	}
	return nil
}

type TxInfoTransfer struct {
	Amount string `json:"amount"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type TxInfoData struct {
	Transfer  TxInfoTransfer `json:"transfer"`
	BlockHash string         `json:"block_hash"`
	ExtIndex  string         `json:"extrinsic_index"`
	Fee       string         `json:"fee"`
	BlockNum  float64        `json:"block_num"`
	BlockTime float64        `json:"block_timestamp"`
}

type TxInfoResponse struct {
	Data TxInfoData `json:"data"`
}

func (client *Client) ParseTxInfo(body []byte) (xc.TxInfo, error) {
	var TxInfoResp TxInfoResponse
	err := json.Unmarshal(body, &TxInfoResp)
	if err != nil {
		return xc.TxInfo{}, err
	}
	if len(TxInfoResp.Data.BlockHash) == 0 {
		return xc.TxInfo{}, errors.New("not found")
	}

	return xc.TxInfo{
		BlockHash: TxInfoResp.Data.BlockHash,
		TxID:      TxInfoResp.Data.ExtIndex,
		From:      xc.Address(TxInfoResp.Data.Transfer.From),
		To:        xc.Address(TxInfoResp.Data.Transfer.To),
		Amount: xc.NewAmountHumanReadableFromStr(TxInfoResp.Data.Transfer.Amount).ToBlockchain(
			client.Asset.GetNativeAsset().Decimals),
		Fee:        xc.NewAmountBlockchainFromStr(TxInfoResp.Data.Fee),
		BlockIndex: int64(TxInfoResp.Data.BlockNum),
		BlockTime:  int64(TxInfoResp.Data.BlockTime),
	}, nil
}

// FetchTxInfo returns tx info for a Substrate tx
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xc.TxInfo, error) {
	if !strings.HasPrefix(string(txHash), "0x") {
		txHash = "0x" + txHash
	}
	var reqBody = []byte(`{"hash": "` + txHash + `"}`)

	asset := client.Asset.GetNativeAsset()
	req, err := http.NewRequest("POST", asset.ExplorerURL+"/api/scan/extrinsic", bytes.NewBuffer(reqBody))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-API-Key", asset.AuthSecret)
	if err != nil {
		return xc.TxInfo{}, err
	}

	explorerClient := &http.Client{}
	resp, err := explorerClient.Do(req)
	if err != nil {
		return xc.TxInfo{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return xc.TxInfo{}, err
	}
	return client.ParseTxInfo(body)
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

	var acctInfo types.AccountInfo
	ok, err := client.DotClient.RPC.State.GetStorageLatest(key, &acctInfo)
	if err != nil || !ok {
		return zero, err
	}

	return xc.NewAmountBlockchainFromUint64(acctInfo.Data.Free.Uint64()), nil
}

// FetchBalance fetches token balance for a Substrate address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	if client.Asset.GetAssetConfig().Asset == client.Asset.GetNativeAsset().Asset {
		return client.FetchNativeBalance(ctx, address)
	} else {
		return xc.AmountBlockchain{}, errors.New("unsupported asset")
	}
}

// Create sample extrinsic with a transaction
func (client *Client) SampleTransaction(ctx context.Context) (xc.Tx, error) {
	sampleAddr := xc.Address(signature.TestKeyringPairAlice.Address)
	txInput, err := client.FetchTxInput(ctx, sampleAddr, sampleAddr)
	if err != nil {
		return &Tx{}, err
	}
	builder, err := NewTxBuilder(client.Asset)
	if err != nil {
		return &Tx{}, err
	}
	tx, err := builder.NewTransfer(sampleAddr, sampleAddr, xc.NewAmountBlockchainFromUint64(1), txInput)
	if err != nil {
		return &Tx{}, err
	}
	sighashes, err := tx.Sighashes()
	if err != nil {
		return &Tx{}, err
	}
	signer, err := NewSigner(client.Asset)
	if err != nil {
		return &Tx{}, err
	}
	signature, err := signer.Sign(xc.PrivateKey(signature.TestKeyringPairAlice.PublicKey), sighashes[0])
	if err != nil {
		return &Tx{}, err
	}
	err = tx.AddSignatures(signature)
	if err != nil {
		return &Tx{}, err
	}
	return tx, nil
}

// EstimateGas estimates the fee for a Substrate transaction (extrinsic)
func (client *Client) EstimateGas(ctx context.Context) (xc.AmountBlockchain, error) {
	// invoke EstimateGasFunc callback, if registered
	if client.EstimateGasFunc != nil {
		nativeAsset := client.Asset.GetNativeAsset().NativeAsset
		res, err := client.EstimateGasFunc(nativeAsset)
		if err != nil {
			// continue with default implementation as fallback
		} else {
			return res, err
		}
	}
	tx, err := client.SampleTransaction(ctx)
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}
	enc, err := tx.Serialize()
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}

	var resp interface{}
	err = client.DotClient.Client.Call(&resp, "payment_queryFeeDetails", codec.HexEncodeToString(enc))
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), err
	}

	fees := resp.(map[string]interface{})["inclusionFee"].(map[string]interface{})
	var total xc.AmountBlockchain
	for _, fee := range fees {
		var feeInt big.Int
		feeInt.SetString(fee.(string), 0)
		total = total.Add((*xc.AmountBlockchain)(&feeInt))
	}
	size := xc.NewAmountBlockchainFromUint64(uint64(len(enc)))
	return total.Div(&size), nil
}

func (client *Client) RegisterEstimateGasCallback(estimateGas xc.EstimateGasFunc) {
	client.EstimateGasFunc = estimateGas
}
