package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	comettypes "github.com/cometbft/cometbft/rpc/core/types"
	jsonrpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/client/localtypes"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	localcodectypes "github.com/cordialsys/crosschain/chain/cosmos/types"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/sirupsen/logrus"

	banktypes "cosmossdk.io/x/bank/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	xc "github.com/cordialsys/crosschain"
	wasmtypes "github.com/cordialsys/crosschain/chain/cosmos/types/CosmWasm/wasmd/x/wasm/types"
	injectiveexchangetypes "github.com/cordialsys/crosschain/chain/cosmos/types/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	xclient "github.com/cordialsys/crosschain/client"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/utils"
	cosmostx "github.com/cosmos/cosmos-sdk/types/tx"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

// Client for Cosmos
type Client struct {
	Asset     xc.ITask
	Ctx       client.Context
	rpcClient *jsonrpcclient.Client
	Prefix    string
}

// At roughly 5s/block this is a ~12 hour timeout.
const TimeoutInBlocks = 10_000

var _ xclient.Client = &Client{}
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
	nativeAsset := xc.NewChainConfig(chain).WithDriver(xc.DriverCosmos).WithUrl(rpcUrl).WithChainPrefix(chainPrefix).WithChainID(chainId)
	return NewClient(nativeAsset)
}

// NewClient returns a new Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	asset := cfgI
	cfg := cfgI.GetChain()
	host := cfg.URL
	interceptor := utils.NewHttpInterceptor(ReplaceIncompatiableCosmosResponses)
	interceptor.Enable()
	rawHttpClient := cfg.DefaultHttpClient()

	// Need to use custom transport because:
	// - cosmos library does not parse URLs correctly
	// - need to intercept responses to remove incompatible response fields for some chains
	rawHttpClient.Transport = interceptor
	httpClient, err := rpchttp.NewWithClient(
		host,
		rawHttpClient,
	)

	if err != nil {
		panic(err)
	}

	// Instantiate also a raw RPC client as we need to re-implement some methods
	// on behalf of special cosmos-sdk chains.
	rawRpcClient, err := jsonrpcclient.NewWithHTTPClient(host, rawHttpClient)
	if err != nil {
		return nil, err
	}
	cosmosCfg, err := localcodectypes.MakeCosmosConfig(cfg.Base())
	if err != nil {
		return nil, err
	}
	cliCtx := client.Context{}.
		WithClient(httpClient).
		WithCodec(cosmosCfg.Marshaler).
		WithTxConfig(cosmosCfg.TxConfig).
		WithLegacyAmino(cosmosCfg.Amino).
		WithInterfaceRegistry(cosmosCfg.InterfaceRegistry).
		WithBroadcastMode("sync").
		WithChainID(string(cfg.ChainID))

	return &Client{
		Asset:     asset,
		Ctx:       cliCtx,
		rpcClient: rawRpcClient,
		Prefix:    string(cfg.ChainPrefix),
	}, nil
}

const DefaultGasLimitMultiplier = 1.2

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	contract, _ := args.GetContract()
	feePayer, _ := args.GetFeePayer()
	baseTxInput, err := client.FetchBaseTxInput(ctx, args.GetFrom(), contract, feePayer)
	if err != nil {
		return nil, err
	}
	res, err := client.Simulate(ctx, *baseTxInput, func(input xc.TxInput) (xc.Tx, error) {
		txBuilder, err := builder.NewTxBuilder(client.Asset.GetChain().Base())
		if err != nil {
			return nil, err
		}
		return txBuilder.Transfer(args, input)
	})
	if err != nil {
		return nil, err
	}
	if res.GasInfo.GasUsed > 0 {
		baseTxInput.GasLimit = res.GasInfo.GasUsed
		// Bump up by 20% generally because cosmos execution can vary a lot.
		gasLimitMultiplier := DefaultGasLimitMultiplier
		if client.Asset.GetChain().ChainGasLimitMultiplier > 0.001 {
			gasLimitMultiplier = client.Asset.GetChain().ChainGasLimitMultiplier
		}
		baseTxInput.GasLimit = uint64(float64(baseTxInput.GasLimit) * gasLimitMultiplier)

		logrus.WithFields(logrus.Fields{
			"gas_limit_multiplier": gasLimitMultiplier,
			"gas_used":             res.GasInfo.GasUsed,
			"from":                 args.GetFrom(),
			"contract":             contract,
		}).Debug("simulated tx")
	}

	return baseTxInput, nil
}

type txBuilder func(input xc.TxInput) (xc.Tx, error)

func (client *Client) Simulate(ctx context.Context, input tx_input.TxInput, txBuilder txBuilder) (*cosmostx.SimulateResponse, error) {
	var simErr error
	for _, gasPrice := range []float64{
		// Try simulation with gas price, as this will produce a more accurate gas limit
		input.GasPrice,
		// simulate without gas price to avoid out-of-balance error
		0,
	} {
		// Note: interestingly, if 0, this will cause the gas limit to lower by 16k gas or 20%
		input.GasPrice = gasPrice

		cosmosTxI, err := txBuilder(&input)
		if err != nil {
			return nil, err
		}
		sigHashes, err := cosmosTxI.Sighashes()
		if err != nil {
			return nil, fmt.Errorf("failed to get sighashes for simulation: %v", err)
		}
		signatures := make([]*xc.SignatureResponse, len(sigHashes))
		for i := range sigHashes {
			signatures[i] = &xc.SignatureResponse{
				Signature: make([]byte, 64),
			}
		}
		cosmosTx := cosmosTxI.(*tx.Tx)
		err = cosmosTx.SetSignatures(signatures...)
		if err != nil {
			return nil, err
		}

		txBz, err := cosmosTx.Serialize()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize tx: %v", err)
		}

		txClient := cosmostx.NewServiceClient(client.Ctx)
		res, err := txClient.Simulate(ctx, &cosmostx.SimulateRequest{
			TxBytes: txBz,
		})
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"gas_price": gasPrice,
				"error":     err,
			}).Warn("failed to simulate tx")
			simErr = err
			continue
		}
		// bump up the gas limit by ~21%, as this is experiementally
		// what I've found to be the difference when omitted gas price.
		if gasPrice == 0 {
			res.GasInfo.GasUsed = uint64(float64(res.GasInfo.GasUsed) * 1.21)
		}

		return res, nil
	}
	// Sometimes the queried account nonce is not final, so we mark this as something
	// that can be retried.
	if simErr != nil && strings.Contains(simErr.Error(), "account sequence mismatch") {
		return nil, errors.FailedPreconditionf("%v", simErr)
	}
	return nil, simErr
}

func (client *Client) FetchBaseTxInput(ctx context.Context, from xc.Address, contractMaybe xc.ContractAddress, feePayerMaybe xc.Address) (*tx_input.TxInput, error) {
	txInput := tx_input.NewTxInput()

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	account, err := client.GetAccount(ctx, from)
	if err != nil || account == nil {
		return txInput, fmt.Errorf("failed to get account data for %v: %v", from, err)
	}
	txInput.AccountNumber = account.GetAccountNumber()
	txInput.Sequence = account.GetSequence()

	if feePayerMaybe != "" {
		feePayerAccount, err := client.GetAccount(ctx, feePayerMaybe)
		if err != nil || feePayerAccount == nil {
			return txInput, fmt.Errorf("failed to get account data for fee-payer %v: %v", feePayerMaybe, err)
		}
		txInput.FeePayerSequence = feePayerAccount.GetSequence()
		txInput.FeePayerAccountNumber = feePayerAccount.GetAccountNumber()
	}

	switch client.Asset.(type) {
	case *xc.ChainConfig:
		txInput.GasLimit = gas.NativeTransferGasLimit
		if client.Asset.GetChain().Chain == xc.HASH {
			txInput.GasLimit = 200_000
		}
	default:
		txInput.GasLimit = gas.TokenTransferGasLimit
	}

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	status, err := client.Ctx.Client.Status(context.Background())
	if err != nil {
		return txInput, fmt.Errorf("could not lookup chain_id: %v", err)
	}
	txInput.ChainId = status.NodeInfo.Network
	txInput.TimeoutHeight = uint64(status.SyncInfo.LatestBlockHeight) + TimeoutInBlocks

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

	_, assetType, err := client.fetchBalanceAndType(ctx, from, contractMaybe)
	if err != nil {
		return txInput, err
	}
	txInput.AssetType = assetType

	return txInput, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// SubmitTx submits a Cosmos tx
func (client *Client) SubmitTx(ctx context.Context, tx1 xctypes.SubmitTxReq) error {
	txBytes, _ := tx1.Serialize()

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
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
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	result := txinfo.LegacyTxInfo{
		Fee:           xc.AmountBlockchain{},
		BlockIndex:    0,
		BlockTime:     0,
		Confirmations: 0,
		TxID:          string(txHash),
	}
	if strings.HasPrefix(string(txHash), "0x") {
		txHash = txHash[2:]
	}

	hash, err := hex.DecodeString(string(txHash))
	if err != nil {
		return result, err
	}

	resultRaw := new(localtypes.ResultTx)

	var hashFormatted interface{} = hash
	switch client.Asset.GetChain().Chain {
	case xc.SEI:
		// Frustratingly, SEI expects the hash as a hex encoded string
		hashFormatted = hex.EncodeToString(hash)
	}

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	_, err = client.rpcClient.Call(ctx, "tx", map[string]interface{}{
		"hash":  hashFormatted,
		"prove": false,
	}, resultRaw)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return result, errors.TransactionNotFoundf("%v", err)
		}
		return result, fmt.Errorf("could not download tx: %v", err)
	}

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	blockResultRaw, err := client.Ctx.Client.Block(ctx, &resultRaw.Height)
	if err != nil {
		return result, err
	}

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	abciInfo, err := client.Ctx.Client.ABCIInfo(ctx)
	if err != nil {
		return result, err
	}
	chainCfg := client.Asset.GetChain()
	memo := ""

	decoder := client.Ctx.TxConfig.TxDecoder()
	{
		decodedTx, err := decoder(resultRaw.Tx)
		if err != nil {
			logrus.WithError(err).Warn("could not decode full transaction")
		} else {
			switch tf := decodedTx.(type) {
			case types.FeeTx:
				result.Fee = xc.AmountBlockchain(*tf.GetFee()[0].Amount.BigInt())
				feePayer, _ := sdk.Bech32ifyAddressBytes(string(client.Asset.GetChain().ChainPrefix), tf.FeePayer())
				result.FeePayer = xc.Address(feePayer)
			default:
				logrus.Warnf("could not determine transaction type for fee %T", tf)
			}
			// Set memo if set
			if withMemo, ok := decodedTx.(types.TxWithMemo); ok {
				memo = withMemo.GetMemo()
			}
		}
	}

	events := ParseEvents(resultRaw.TxResult.Events)
	for _, fee := range events.Fees {
		result.Fee = fee.Amount
		result.FeeContract = xc.ContractAddress(fee.Contract)
		if result.FeeContract == xc.ContractAddress(client.Asset.GetChain().ChainCoin) {
			// same as native asset
			result.FeeContract = ""
		}
		result.FeePayer = xc.Address(fee.Payer)
	}
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
		result.Sources = append(result.Sources, &txinfo.LegacyTxInfoEndpoint{
			Address:         xc.Address(ev.Sender),
			ContractAddress: xc.ContractAddress(contract),
			ContractId:      xc.ContractAddress(altContractId),
			Amount:          ev.Amount,
			Event:           txinfo.NewEventFromIndex(uint64(ev.Index), txinfo.MovementVariantNative),
		})
		result.Destinations = append(result.Destinations, &txinfo.LegacyTxInfoEndpoint{
			Address:         xc.Address(ev.Recipient),
			ContractAddress: xc.ContractAddress(contract),
			ContractId:      xc.ContractAddress(altContractId),
			Amount:          ev.Amount,
			Event:           txinfo.NewEventFromIndex(uint64(ev.Index), txinfo.MovementVariantNative),
		})
	}
	for _, ev := range events.Delegates {
		result.AddStakeEvent(&txinfo.Stake{
			Balance:   ev.Amount,
			Validator: ev.Validator,
			Account:   "",
			Address:   ev.Delegator,
		})
	}
	for _, ev := range events.Unbonds {
		result.AddStakeEvent(&txinfo.Unstake{
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
	for _, dst := range result.Destinations {
		dst.Memo = memo
	}

	result.BlockIndex = resultRaw.Height
	result.BlockTime = blockResultRaw.Block.Header.Time.Unix()
	result.Confirmations = abciInfo.Response.LastBlockHeight - result.BlockIndex

	if resultRaw.TxResult.Code != 0 {
		result.Status = xc.TxStatusFailure
		result.Error = resultRaw.TxResult.Log
		// drop movements
		result.Sources = nil
		result.Destinations = nil
		result.ResetStakeEvents()
	}

	return result, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHashStr := args.TxHash()
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// remap to new tx
	return txinfo.TxInfoFromLegacy(chain, legacyTx, txinfo.Account), nil
}

// GetAccount returns a Cosmos account
// Equivalent to client.Ctx.AccountRetriever.GetAccount(), but doesn't rely GetConfig()
func (client *Client) GetAccount(ctx context.Context, address xc.Address) (client.Account, error) {
	_ = client.Asset.GetChain().Limiter.Wait(ctx)
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
func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	contract, _ := args.Contract()
	bal, _, err := client.fetchBalanceAndType(ctx, args.Address(), contract)
	return bal, err
}

func (client *Client) fetchBalanceAndType(ctx context.Context, address xc.Address, contractMaybe xc.ContractAddress) (xc.AmountBlockchain, tx_input.CosmoAssetType, error) {
	// attempt getting the x/bank module balance first.
	bal, bankErr := client.fetchBankModuleBalance(ctx, address, contractMaybe)
	if bankErr == nil {
		if bal.Uint64() == 0 {
			// sometimes x/bank will incorrectly return 0 balance for invalid bank assets (like on terra chain).
			// so if there's 0 bal, we double check if there's an cw20 balance.
			bal, cw20Err := client.FetchCw20Balance(ctx, address, string(contractMaybe))
			if cw20Err == nil && bal.Uint64() > 0 {
				return bal, tx_input.CW20, nil
			}
		}
		return bal, tx_input.BANK, nil
	}

	// attempt getting the cw20 balance.
	bal, cw20Err := client.FetchCw20Balance(ctx, address, string(contractMaybe))
	if cw20Err == nil {
		return bal, tx_input.CW20, nil
	}

	return bal, "", fmt.Errorf("could not determine balance for bank (%v) or cw20 (%v)", bankErr, cw20Err)
}

func (client *Client) FetchCw20Balance(ctx context.Context, address xc.Address, contract string) (xc.AmountBlockchain, error) {
	_ = client.Asset.GetChain().Limiter.Wait(ctx)
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

// Cosmos chains can have multiple native assets.  This helper is necessary to query the
// native bank module for a given asset.
func (client *Client) fetchBankModuleBalance(ctx context.Context, address xc.Address, contractMaybe xc.ContractAddress) (xc.AmountBlockchain, error) {
	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	zero := xc.NewAmountBlockchainFromUint64(0)

	_, err := types.GetFromBech32(string(address), client.Prefix)
	if err != nil {
		return zero, fmt.Errorf("bad address: '%v': %v", address, err)
	}
	denom := ""
	// denom should be the contract if it's set.
	denom = string(contractMaybe)
	if denom == "" {
		// use the default chain coin (should be set for cosmos chains)
		denom = client.Asset.GetChain().ChainCoin
	}

	if denom == "" {
		return zero, fmt.Errorf("failed to account balance: no denom on asset")
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
	// check chain-coin if it's set
	chainCfg := client.Asset.GetChain()
	if chainCfg.ChainCoin == string(contract) {
		return int(chainCfg.Decimals), nil
	}
	// check additional native assets
	for _, asset := range chainCfg.NativeAssets {
		if asset.AssetId == string(contract) || asset.ContractId == contract {
			return int(asset.Decimals), nil
		}
	}

	_ = client.Asset.GetChain().Limiter.Wait(ctx)
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
				Name   string `json:"name"`
				Symbol string `json:"symbol"`
				// tolerate int being encoded number or string
				Decimals    xc.AmountBlockchain `json:"decimals"`
				TotalSupply xc.AmountBlockchain `json:"total_supply"`
			}
			var tokenInfo TokenInfoResponse

			_ = client.Asset.GetChain().Limiter.Wait(ctx)
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
				return int(tokenInfo.Decimals.Uint64()), nil
			}
		}
		// Try lookup injective peggy asset
		{
			_ = client.Asset.GetChain().Limiter.Wait(ctx)
			injectiveQ := injectiveexchangetypes.NewQueryClient(client.Ctx)
			injectiveResponse, err := injectiveQ.DenomDecimal(ctx, &injectiveexchangetypes.QueryDenomDecimalRequest{
				Denom: string(contract),
			})

			if err == nil {
				return int(injectiveResponse.Decimal), nil
			}
		}

		if strings.HasPrefix(string(contract), "ibc/") {
			// Sometimes IBC assets don't have any registered metadata.
			// Default to 6.
			return 6, nil
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

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	_ = client.Asset.GetChain().Limiter.Wait(ctx)
	var cometBlock *comettypes.ResultBlock
	var err error
	height, ok := args.Height()
	if !ok {
		cometBlock, err = client.Ctx.Client.Block(ctx, nil)
	} else {
		h := int64(height)
		cometBlock, err = client.Ctx.Client.Block(ctx, &h)
	}
	if err != nil {
		return nil, err
	}

	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			uint64(cometBlock.Block.Height),
			cometBlock.BlockID.Hash.String(),
			cometBlock.Block.Time,
		),
		TransactionIds: []string{},
	}
	for _, tx := range cometBlock.Block.Txs {
		if client.Asset.GetChain().Chain == xc.TIA {
			// this is pretty hacky, but otherwise the standard way of producing the hash isn't correct.
			// Not sure how else to calculate the hash, and can't find an efficient way to otherwise figure out what it is.
			if strings.Contains(string(tx), "/celestia.blob.v1.MsgPayForBlobs") {
				// just drop it
				continue
			}
		}
		block.TransactionIds = append(block.TransactionIds, hex.EncodeToString(tx.Hash()))
	}

	return block, nil

}
