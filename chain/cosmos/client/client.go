package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	comettypes "github.com/cometbft/cometbft/rpc/core/types"
	jsonrpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	localcodectypes "github.com/cordialsys/crosschain/chain/cosmos/types"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/sirupsen/logrus"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	xc "github.com/cordialsys/crosschain"
	wasmtypes "github.com/cordialsys/crosschain/chain/cosmos/types/CosmWasm/wasmd/x/wasm/types"
	injectiveexchangetypes "github.com/cordialsys/crosschain/chain/cosmos/types/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/utils"

	// injectivecryptocodec "github.com/InjectiveLabs/sdk-go/chain/crypto/codec"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

// Client for Cosmos
type Client struct {
	Asset     xc.ITask
	Ctx       client.Context
	rpcClient *jsonrpcclient.Client
	Prefix    string
}

var _ xclient.FullClient = &Client{}
var _ xclient.ClientWithDecimals = &Client{}
var _ xclient.StakingClient = &Client{}

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

	rawHttpClient := &http.Client{
		// Need to use custom transport because:
		// - cosmos library does not parse URLs correctly
		// - need to intercept responses to remove incompatible response fields for some chains
		Transport: interceptor,
	}
	httpClient, err := rpchttp.NewWithClient(
		host,
		"websocket",
		rawHttpClient,
	)

	if err != nil {
		panic(err)
	}
	_ = httpClient

	// Instantiate also a raw RPC client as we need to re-implement some methods
	// on behalf of special cosmos-sdk chains.
	rawRpcClient, err := jsonrpcclient.NewWithHTTPClient(host, rawHttpClient)
	if err != nil {
		return nil, err
	}
	cosmosCfg := localcodectypes.MakeCosmosConfig()
	cliCtx := client.Context{}.
		WithClient(httpClient).
		WithCodec(cosmosCfg.Marshaler).
		WithTxConfig(cosmosCfg.TxConfig).
		WithLegacyAmino(cosmosCfg.Amino).
		WithInterfaceRegistry(cosmosCfg.InterfaceRegistry).
		WithBroadcastMode("sync").
		WithChainID(string(cfg.ChainIDStr))

	return &Client{
		Asset:     asset,
		Ctx:       cliCtx,
		rpcClient: rawRpcClient,
		Prefix:    cfg.ChainPrefix,
	}, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	baseTxInput, err := client.FetchBaseTxInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	return baseTxInput, nil
}

func (client *Client) FetchBaseTxInput(ctx context.Context, from xc.Address) (*tx_input.TxInput, error) {
	txInput := tx_input.NewTxInput()

	account, err := client.GetAccount(ctx, from)
	if err != nil || account == nil {
		return txInput, fmt.Errorf("failed to get account data for %v: %v", from, err)
	}
	txInput.AccountNumber = account.GetAccountNumber()
	txInput.Sequence = account.GetSequence()
	switch client.Asset.(type) {
	case *xc.ChainConfig:
		txInput.GasLimit = gas.NativeTransferGasLimit
		if client.Asset.GetChain().Chain == xc.HASH {
			txInput.GasLimit = 200_000
		}
	default:
		txInput.GasLimit = gas.TokenTransferGasLimit
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

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Cosmos tx
func (client *Client) SubmitTx(ctx context.Context, tx1 xc.Tx) error {
	txBytes, _ := tx1.Serialize()

	res, err := client.Ctx.BroadcastTx(txBytes)
	if err != nil {
		return errors.Unknownf("%v", err)
	}

	if res.Code != 0 {
		txID := tx.TmHash(txBytes)
		// Code for already in mempool
		if res.Code == 19 {
			return errors.TransactionExistsf("tx %v failed code: %v, log: %v", txID, res.Code, res.RawLog)
		}
		return errors.Unknownf("tx %v failed code: %v, log: %v", txID, res.Code, res.RawLog)
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

	resultRaw := new(comettypes.ResultTx)

	var hashFormatted interface{} = hash
	switch client.Asset.GetChain().Chain {
	case xc.SEI:
		// Frustratingly, SEI expects the hash as a hex encoded string
		hashFormatted = hex.EncodeToString(hash)
	}

	_, err = client.rpcClient.Call(ctx, "tx", map[string]interface{}{
		"hash":  hashFormatted,
		"prove": false,
	}, resultRaw)
	if err != nil {
		return result, fmt.Errorf("could not download tx: %v", err)
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

	tx := &tx.Tx{
		CosmosTx:        decodedTx,
		CosmosTxEncoder: client.Ctx.TxConfig.TxEncoder(),
	}
	chainCfg := client.Asset.GetChain()
	result.TxID = string(txHash)
	result.Fee = tx.Fee()

	events := ParseEvents(resultRaw.TxResult.Events)
	for _, ev := range events.Transfers {
		contract := ev.Contract
		// Assets on cosmos chains techically always have a contract value ("denom") that is not
		// empty.  This conflicts with our assignment of "chains/<CHAIN>/assets/<CHAIN>" ID.
		// To provide a consistent output, we catch the right "denom" and convert it to our ID.
		altContractId := ""
		if contract == chainCfg.ChainCoin {
			altContractId = contract
			contract = string(chainCfg.Chain)
		}
		result.Sources = append(result.Sources, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(ev.Sender),
			ContractAddress: xc.ContractAddress(contract),
			ContractId:      xc.ContractAddress(altContractId),
			Amount:          ev.Amount,
		})
		result.Destinations = append(result.Destinations, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(ev.Recipient),
			ContractAddress: xc.ContractAddress(contract),
			ContractId:      xc.ContractAddress(altContractId),
			Amount:          ev.Amount,
		})
	}
	for _, ev := range events.Delegates {
		result.AddStakeEvent(&xclient.Stake{
			Balance:   ev.Amount,
			Validator: ev.Validator,
			Account:   "",
			Address:   ev.Delegator,
		})
	}
	for _, ev := range events.Unbonds {
		result.AddStakeEvent(&xclient.Unstake{
			Balance:   ev.Amount,
			Validator: ev.Validator,
			Account:   "",
			Address:   ev.Delegator,
		})
	}

	if len(result.Sources) > 0 {
		result.From = result.Sources[0].Address
		result.Amount = result.Sources[0].Amount
		result.ContractAddress = result.Sources[0].ContractAddress
	}
	if len(result.Destinations) > 0 {
		result.To = result.Destinations[0].Address
		result.Amount = result.Destinations[0].Amount
		result.ContractAddress = result.Destinations[0].ContractAddress
	}

	// Set memo if set
	if withMemo, ok := decodedTx.(types.TxWithMemo); ok {
		memo := withMemo.GetMemo()
		for _, dst := range result.Destinations {
			dst.Memo = memo
		}
	}

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
	chain := client.Asset.GetChain()

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

func (client *Client) fetchBalanceAndType(ctx context.Context, address xc.Address) (xc.AmountBlockchain, tx_input.CosmoAssetType, error) {
	// attempt getting the x/bank module balance first.
	bal, bankErr := client.fetchBankModuleBalance(ctx, address, client.Asset)
	if bankErr == nil {
		if bal.Uint64() == 0 {
			// sometimes x/bank will incorrectly return 0 balance for invalid bank assets (like on terra chain).
			// so if there's 0 bal, we double check if there's an cw20 balance.
			bal, cw20Err := client.FetchCw20Balance(ctx, address, client.Asset.GetContract())
			if cw20Err == nil && bal.Uint64() > 0 {
				return bal, tx_input.CW20, nil
			}
		}
		return bal, tx_input.BANK, nil
	}

	// attempt getting the cw20 balance.
	bal, cw20Err := client.FetchCw20Balance(ctx, address, client.Asset.GetContract())
	if cw20Err == nil {
		return bal, tx_input.CW20, nil
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
		Balance string `json:"balance"`
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

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}
	queryClient := banktypes.NewQueryClient(client.Ctx)
	denomMetaResponse, bankErr := queryClient.DenomMetadata(ctx, &banktypes.QueryDenomMetadataRequest{
		Denom: string(contract),
	})
	if bankErr != nil {
		logrus.WithError(bankErr).Debug("not a bank asset")
		// Try lookup cw20
		{
			input := json.RawMessage(`{"token_info": {}}`)
			type TokenInfoResponse struct {
				Name        string `json:"name"`
				Symbol      string `json:"symbol"`
				Decimals    int64  `json:"decimals"`
				TotalSupply int64  `json:"total_supply"`
			}
			var tokenInfo TokenInfoResponse

			tokenResp, err := wasmtypes.NewQueryClient(client.Ctx).SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
				QueryData: wasmtypes.RawContractMessage(input),
				Address:   string(contract),
			})

			if err == nil {
				logrus.WithField("response", string(tokenResp.Data.Bytes())).Debug("cw20 asset")
				err = json.Unmarshal(tokenResp.Data.Bytes(), &tokenInfo)
				if err != nil {
					return 0, fmt.Errorf("failed to parse cw20 token info: '%v': %v", contract, err)
				}
				return int(tokenInfo.Decimals), nil
			}
		}
		// Try lookup injective peggy asset
		{
			injectiveQ := injectiveexchangetypes.NewQueryClient(client.Ctx)
			injectiveResponse, err := injectiveQ.DenomDecimal(ctx, &injectiveexchangetypes.QueryDenomDecimalRequest{
				Denom: string(contract),
			})

			if err == nil {
				return int(injectiveResponse.Decimal), nil
			}
		}

		// return original bank error
		return 0, bankErr
	}
	bz, _ := json.Marshal(denomMetaResponse.Metadata)
	logrus.WithField("response", string(bz)).Debug("bank asset")

	// The asset may be reported with a bunch of shorthand aliases with different exponents.
	// We'll take the highest one, assuming that must be the difference from the machine amount.
	maxDecimal := 0
	for _, denom := range denomMetaResponse.Metadata.DenomUnits {
		if denom.Exponent > uint32(maxDecimal) {
			maxDecimal = int(denom.Exponent)
		}
	}
	return maxDecimal, nil
}
