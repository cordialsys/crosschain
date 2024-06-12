package cosmos

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/shopspring/decimal"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/utils"

	// injectivecryptocodec "github.com/InjectiveLabs/sdk-go/chain/crypto/codec"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

type CosmoAssetType string

// Cosmos assets can be managed by completely different modules (e.g. cosmwasm cw20, x/bank, etc)
var CW20 CosmoAssetType = "cw20"
var BANK CosmoAssetType = "bank"

// TxInput for Cosmos
type TxInput struct {
	xc.TxInputEnvelope
	AccountNumber uint64  `json:"account_number,omitempty"`
	Sequence      uint64  `json:"sequence,omitempty"`
	GasLimit      uint64  `json:"gas_limit,omitempty"`
	GasPrice      float64 `json:"gas_price,omitempty"`
	Memo          string  `json:"memo,omitempty"`
	FromPublicKey []byte  `json:"from_pubkey,omitempty"`

	AssetType CosmoAssetType `json:"asset_type,omitempty"`
	ChainId   string         `json:"chain_id,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}
var _ xc.TxInputWithMemo = &TxInput{}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	input.GasPrice, _ = multiplier.Mul(decimal.NewFromFloat(input.GasPrice)).Float64()
	return nil
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if cosmosOther, ok := other.(*TxInput); ok {
		// cosmos address could own multiple accounts too which each have independent sequence.
		return cosmosOther.AccountNumber != input.AccountNumber ||
			cosmosOther.Sequence != input.Sequence
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
func (txInput *TxInput) SetPublicKey(publicKeyBytes xc.PublicKey) error {
	txInput.FromPublicKey = publicKeyBytes
	return nil
}

func (txInput *TxInput) SetMemo(memo string) {
	txInput.Memo = memo
}

func (txInput *TxInput) SetPublicKeyFromStr(publicKeyStr string) error {
	var publicKeyBytes []byte
	var err error
	if strings.HasPrefix(publicKeyStr, "0x") {
		publicKeyBytes, err = hex.DecodeString(publicKeyStr)
	} else {
		publicKeyBytes, err = base64.StdEncoding.DecodeString(publicKeyStr)
	}
	if err != nil {
		return fmt.Errorf("invalid public key %v: %v", publicKeyStr, err)
	}
	return txInput.SetPublicKey(publicKeyBytes)
}

// NewTxInput returns a new Cosmos TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverCosmos),
	}
}

// Client for Cosmos
type Client struct {
	Asset  xc.ITask
	Ctx    client.Context
	Prefix string
}

var _ xclient.FullClient = &Client{}

func ReplaceIncompatiableCosmosResponses(body []byte) []byte {
	bodyStr := string(body)

	// Output traces:
	// data := map[string]interface{}{}
	// json.Unmarshal(body, &data)
	// bz, _ := json.Marshal(data)
	// fmt.Println("", string(bz))

	// try to parse as json and remove .result.block.evidence field as it's incompatible between chains
	// by just renaming the key it should just get dropped during parsing
	bodyStr = strings.Replace(bodyStr, "\"evidence\"", "\"_evidence\"", 1)

	return []byte(bodyStr)
}

func NewClientFrom(chain xc.NativeAsset, chainId string, chainPrefix string, rpcUrl string) (*Client, error) {

	nativeAsset := &xc.ChainConfig{
		Chain:       chain,
		Driver:      xc.DriverCosmos,
		URL:         rpcUrl,
		ChainPrefix: chainPrefix,
		ChainIDStr:  chainId,
	}
	return NewClient(nativeAsset)
}

// NewClient returns a new Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	host := cfg.URL
	interceptor := utils.NewHttpInterceptor(ReplaceIncompatiableCosmosResponses)
	interceptor.Enable()
	httpClient, err := rpchttp.NewWithClient(
		host,
		"websocket",
		&http.Client{
			// Need to use custom transport because:
			// - cosmos library does not parse URLs correctly
			// - need to intercept responses to remove incompatible response fields for some chains
			Transport: interceptor,
		})
	if err != nil {
		panic(err)
	}
	_ = httpClient
	cosmosCfg := MakeCosmosConfig()
	cliCtx := client.Context{}.
		WithClient(httpClient).
		WithCodec(cosmosCfg.Marshaler).
		WithTxConfig(cosmosCfg.TxConfig).
		WithLegacyAmino(cosmosCfg.Amino).
		WithInterfaceRegistry(cosmosCfg.InterfaceRegistry).
		WithBroadcastMode("sync").
		WithChainID(string(cfg.ChainIDStr))

	return &Client{
		Asset:  asset,
		Ctx:    cliCtx,
		Prefix: cfg.ChainPrefix,
	}, nil
}

// FetchTxInput returns tx input for a Cosmos tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, _ xc.Address) (xc.TxInput, error) {
	txInput := NewTxInput()

	account, err := client.GetAccount(ctx, from)
	if err != nil || account == nil {
		return txInput, fmt.Errorf("failed to get account data for %v: %v", from, err)
	}
	txInput.AccountNumber = account.GetAccountNumber()
	txInput.Sequence = account.GetSequence()
	switch client.Asset.(type) {
	case *xc.ChainConfig:
		txInput.GasLimit = NativeTransferGasLimit
		if client.Asset.GetChain().Chain == xc.HASH {
			txInput.GasLimit = 200_000
		}
	default:
		txInput.GasLimit = TokenTransferGasLimit
	}

	status, err := client.Ctx.Client.Status(context.Background())
	if err != nil {
		return txInput, fmt.Errorf("could not lookup chain_id: %v", err)
	}
	txInput.ChainId = status.NodeInfo.Network

	if !client.Asset.GetChain().NoGasFees {
		gasPrice, err := client.EstimateGasPrice(ctx)
		if err != nil {
			return txInput, fmt.Errorf("failed to estimate gas: %v", err)
		}
		if mult := client.Asset.GetChain().ChainGasMultiplier; mult > 0 {
			gasPrice = gasPrice * mult
		}
		txInput.GasPrice = gasPrice
	}

	_, assetType, err := client.fetchBalanceAndType(ctx, from)
	if err != nil {
		return txInput, err
	}
	txInput.AssetType = assetType

	return txInput, nil
}

// SubmitTx submits a Cosmos tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	txBytes, _ := tx.Serialize()

	res, err := client.Ctx.BroadcastTx(txBytes)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx %v", err)
	}

	if res.Code != 0 {
		txID := tmHash(txBytes)
		return fmt.Errorf("tx %v failed code: %v, log: %v", txID, res.Code, res.RawLog)
	}

	return nil
}

// FetchLegacyTxInfo returns tx info for a Cosmos tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	result := xc.LegacyTxInfo{
		Fee:           xc.AmountBlockchain{},
		BlockIndex:    0,
		BlockTime:     0,
		Confirmations: 0,
	}
	if strings.HasPrefix(string(txHash), "0x") {
		txHash = txHash[2:]
	}

	hash, err := hex.DecodeString(string(txHash))
	if err != nil {
		return result, err
	}

	resultRaw, err := client.Ctx.Client.Tx(ctx, hash, false)
	if err != nil {
		return result, err
	}

	blockResultRaw, err := client.Ctx.Client.Block(ctx, &resultRaw.Height)
	if err != nil {
		return result, err
	}

	abciInfo, err := client.Ctx.Client.ABCIInfo(ctx)
	if err != nil {
		return result, err
	}

	decoder := client.Ctx.TxConfig.TxDecoder()
	decodedTx, err := decoder(resultRaw.Tx)
	if err != nil {
		return result, err
	}

	tx := &Tx{
		CosmosTx:        decodedTx,
		CosmosTxEncoder: client.Ctx.TxConfig.TxEncoder(),
	}

	result.TxID = string(txHash)
	result.ExplorerURL = client.Asset.GetChain().ExplorerURL + "/tx/" + result.TxID
	tx.ParseTransfer()

	// parse tx info - this should happen after ATA is set
	// (in most cases it works also in case or error)
	result.From = tx.From()
	result.To = tx.To()
	result.ContractAddress = tx.ContractAddress()
	result.Amount = tx.Amount()
	result.Fee = tx.Fee()
	result.Sources = tx.Sources()
	result.Destinations = tx.Destinations()

	result.BlockIndex = resultRaw.Height
	result.BlockTime = blockResultRaw.Block.Header.Time.Unix()
	result.Confirmations = abciInfo.Response.LastBlockHeight - result.BlockIndex

	if resultRaw.TxResult.Code != 0 {
		result.Status = xc.TxStatusFailure
	}

	return result, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain().Chain

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Account), nil
}

// GetAccount returns a Cosmos account
// Equivalent to client.Ctx.AccountRetriever.GetAccount(), but doesn't rely GetConfig()
func (client *Client) GetAccount(ctx context.Context, address xc.Address) (client.Account, error) {
	_, err := types.GetFromBech32(string(address), client.Prefix)
	if err != nil {
		return nil, fmt.Errorf("bad address: '%v': %v", address, err)
	}

	res, err := authtypes.NewQueryClient(client.Ctx).Account(ctx, &authtypes.QueryAccountRequest{Address: string(address)})
	if err != nil {
		return nil, err
	}

	var acc authtypes.AccountI
	if err := client.Ctx.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
		return nil, err
	}
	return acc, nil
}

// FetchBalance fetches balance for input asset for a Cosmos address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	bal, _, err := client.fetchBalanceAndType(ctx, address)
	return bal, err
}

func (client *Client) fetchBalanceAndType(ctx context.Context, address xc.Address) (xc.AmountBlockchain, CosmoAssetType, error) {
	// attempt getting the x/bank module balance first.
	bal, bankErr := client.fetchBankModuleBalance(ctx, address, client.Asset)
	if bankErr == nil {
		if bal.Uint64() == 0 {
			// sometimes x/bank will incorrectly return 0 balance for invalid bank assets (like on terra chain).
			// so if there's 0 bal, we double check if there's an cw20 balance.
			bal, cw20Err := client.FetchCw20Balance(ctx, address, client.Asset.GetContract())
			if cw20Err == nil && bal.Uint64() > 0 {
				return bal, CW20, nil
			}
		}
		return bal, BANK, nil
	}

	// attempt getting the cw20 balance.
	bal, cw20Err := client.FetchCw20Balance(ctx, address, client.Asset.GetContract())
	if cw20Err == nil {
		return bal, CW20, nil
	}

	return bal, "", fmt.Errorf("could not determine balance for bank (%v) or cw20 (%v)", bankErr, cw20Err)
}

func (client *Client) FetchCw20Balance(ctx context.Context, address xc.Address, contract string) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	contractAddress := contract

	_, err := types.GetFromBech32(string(address), client.Prefix)
	if err != nil {
		return zero, fmt.Errorf("bad address: '%v': %v", address, err)
	}

	input := json.RawMessage(`{"balance": {"address": "` + string(address) + `"}}`)
	type TokenBalance struct {
		Balance string
	}
	var balResult TokenBalance

	balResp, err := wasmtypes.NewQueryClient(client.Ctx).SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
		QueryData: wasmtypes.RawContractMessage(input),
		Address:   contractAddress,
	})
	if err != nil {
		return zero, fmt.Errorf("failed to get token balance: '%v': %v", address, err)
	}
	err = json.Unmarshal(balResp.Data.Bytes(), &balResult)
	if err != nil {
		return zero, fmt.Errorf("failed to parse token balance: '%v': %v", address, err)
	}

	balance := xc.NewAmountBlockchainFromStr(balResult.Balance)
	return balance, nil
}

// FetchNativeBalance fetches account balance for a Cosmos address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.fetchBankModuleBalance(ctx, address, client.Asset.GetChain())
}

// Cosmos chains can have multiple native assets.  This helper is necessary to query the
// native bank module for a given asset.
func (client *Client) fetchBankModuleBalance(ctx context.Context, address xc.Address, asset xc.ITask) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	_, err := types.GetFromBech32(string(address), client.Prefix)
	if err != nil {
		return zero, fmt.Errorf("bad address: '%v': %v", address, err)
	}
	denom := ""
	// denom should be the contract if it's set.
	denom = client.Asset.GetContract()
	if denom == "" {
		// use the default chain coin (should be set for cosmos chains)
		denom = client.Asset.GetChain().ChainCoin
	}

	if denom == "" {
		return zero, fmt.Errorf("failed to account balance: no denom on asset %s", asset.ID())
	}

	queryClient := banktypes.NewQueryClient(client.Ctx)
	balResp, err := queryClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: string(address),
		Denom:   denom,
	})
	if err != nil {
		if strings.Contains(err.Error(), "invalid denom") {
			// Some chains do not properly support getting balance by denom directly, but will support when getting all of the balances.
			allBals, err := queryClient.AllBalances(ctx, &banktypes.QueryAllBalancesRequest{
				Address: string(address),
				Pagination: &query.PageRequest{
					Limit: 100,
				},
			})
			if err != nil {
				return zero, fmt.Errorf("failed to get any account balance: '%v': %v", address, err)
			}
			for _, bal := range allBals.Balances {
				if bal.Denom == denom {
					return xc.AmountBlockchain(*bal.Amount.BigInt()), nil
				}
			}
		}
		return zero, fmt.Errorf("failed to get account balance: '%v': %v", address, err)
	}
	if balResp == nil || balResp.GetBalance() == nil {
		return zero, fmt.Errorf("failed to get account balance: '%v': %v", address, err)
	}
	balance := balResp.GetBalance().Amount.BigInt()
	return xc.AmountBlockchain(*balance), nil
}
