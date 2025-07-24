package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/erc20"
	"github.com/cordialsys/crosschain/chain/evm/abi/exit_request"
	"github.com/cordialsys/crosschain/chain/evm/abi/stake_deposit"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/client/rpctypes"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/cordialsys/crosschain/normalize"
	"github.com/cordialsys/crosschain/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
)

const DEFAULT_GAS_PRICE = 20_000_000_000
const DEFAULT_GAS_TIP = 3_000_000_000

var ERC20 abi.ABI

func init() {
	var err error
	ERC20, err = abi.JSON(strings.NewReader(erc20.Erc20ABI))
	if err != nil {
		panic(err)
	}
}

// Client for EVM
type Client struct {
	Asset       xc.ITask
	EthClient   *ethclient.Client
	ChainId     *big.Int
	Interceptor *utils.HttpInterceptor
}

var _ xclient.Client = &Client{}
var _ xclient.MultiTransferClient = &Client{}

// Ethereum does not support full delegated staking, so we can only report balance information.
// A 3rd party 'staking provider' is required to do the rest.
var _ xclient.StakingClient = &Client{}

func ReplaceIncompatiableEvmResponses(body []byte) []byte {
	bodyStr := string(body)
	newStr := ""
	// KLAY issue
	if strings.Contains(bodyStr, "type\":\"TxTypeLegacyTransaction") {
		log.Print("Replacing KLAY TxTypeLegacyTransaction")
		newStr = strings.Replace(bodyStr, "TxTypeLegacyTransaction", "0x0", 1)
		newStr = strings.Replace(newStr, "\"V\"", "\"v\"", 1)
		newStr = strings.Replace(newStr, "\"R\"", "\"r\"", 1)
		newStr = strings.Replace(newStr, "\"S\"", "\"s\"", 1)
		newStr = strings.Replace(newStr, "\"signatures\":[{", "", 1)
		newStr = strings.Replace(newStr, "}]", ",\"cumulativeGasUsed\":\"0x0\"", 1)
	}
	if strings.Contains(bodyStr, "parentHash") {
		log.Print("Adding KLAY/CELO sha3Uncles")
		newStr = strings.Replace(bodyStr, "parentHash", "gasLimit\":\"0x0\",\"difficulty\":\"0x0\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"sha3Uncles\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"parentHash", 1)
	}
	if newStr == "" {
		newStr = bodyStr[:]
	}
	if strings.Contains(bodyStr, "\"xdc") {
		log.Print("Replacing xdc prefix with 0x")
		newStr = strings.Replace(newStr, "\"xdc", "\"0x", -1)
	}

	if newStr != "" {
		return []byte(newStr)
	}
	// return unmodified body
	return body
}

// NewClient returns a new EVM Client
func NewClient(asset xc.ITask) (*Client, error) {
	nativeAsset := asset.GetChain()
	url := asset.GetChain().URL

	// c, err := rpc.DialContext(context.Background(), url)
	interceptor := utils.NewHttpInterceptor(ReplaceIncompatiableEvmResponses)

	httpClient := asset.GetChain().DefaultHttpClient()
	httpClient.Transport = interceptor
	c, err := rpc.DialHTTPWithClient(url, httpClient)
	if err != nil {
		return nil, fmt.Errorf("dialing url: %v", nativeAsset.URL)
	}

	client := ethclient.NewClient(c)
	return &Client{
		Asset:       asset,
		EthClient:   client,
		ChainId:     nil,
		Interceptor: interceptor,
	}, nil
}

// SubmitTx submits a EVM tx
func (client *Client) SubmitTx(ctx context.Context, tx xc.Tx) error {
	bz, err := tx.Serialize()
	if err != nil {
		return err
	}
	bzHex := hexutil.Encode(bz)
	err1 := client.EthClient.Client().CallContext(ctx, nil, "eth_sendRawTransaction", bzHex)

	secondaryUrl := client.Asset.GetChain().SecondaryURL
	if secondaryUrl != "" {
		// We support submitting to multiple RPC nodes.  With some evm clients, they have poor performance
		// when it comes to rebroadcasting transactions.  For example, Geth seems to cache transactions
		// for a while when they don't have enough funds to pay for gas.  When funds later loaded, Geth will
		// silently ignore valid rebroadcasts for like 20+ minutes.  By submitting to an additional non-geth node, we can
		// hopefully avoid this issue.
		secondaryClient, err := rpc.DialHTTPWithClient(secondaryUrl, http.DefaultClient)
		if err != nil {
			return fmt.Errorf("dialing url: %v", secondaryUrl)
		}
		err2 := secondaryClient.CallContext(ctx, nil, "eth_sendRawTransaction", bzHex)
		if err1 == nil || err2 == nil {
			// If one succeeds, we're good.
			return nil
		}
	}
	return err1
}

// FetchLegacyTxInfo returns tx info for a EVM tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.LegacyTxInfo, error) {
	nativeAsset := client.Asset.GetChain()
	txHashHex := address.TrimPrefixes(string(txHashStr))
	txHash := common.HexToHash(txHashHex)

	result := xclient.LegacyTxInfo{
		TxID: txHashHex,
	}

	var trans = &rpctypes.Transaction{}
	err := client.EthClient.Client().CallContext(ctx, trans, "eth_getTransactionByHash", txHash.String())
	if err == nil && len(trans.Hash) == 0 {
		err = fmt.Errorf("not found")
	}
	if err != nil {
		// evm returns a simple string for not found condition
		if strings.ToLower(err.Error()) == "not found" || len(trans.Hash) == 0 {
			return result, errors.TransactionNotFoundf("%v", err)
		}
		return result, fmt.Errorf("fetching tx by hash '%s': %v", txHashStr, err)
	}

	// If the transaction is still pending, return an empty txInfo.
	if trans.BlockNumber.Uint64() == 0 {
		return result, nil
	}

	receipt, err := client.EthClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		// TODO retry only for KLAY
		client.Interceptor.Enable()
		receipt, err = client.EthClient.TransactionReceipt(ctx, txHash)
		client.Interceptor.Disable()
		if err != nil {
			return result, fmt.Errorf("fetching receipt for tx %v : %v", txHashStr, err)
		}
	}

	// if no receipt, tx has 0 confirmations
	if receipt == nil {
		return result, nil
	}

	result.BlockIndex = receipt.BlockNumber.Int64()
	result.BlockHash = receipt.BlockHash.Hex()
	gasUsed := receipt.GasUsed
	if receipt.Status == 0 {
		result.Status = xc.TxStatusFailure
		result.Error = "transaction reverted"
	}

	// tx confirmed
	currentHeader, err := client.EthClient.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		client.Interceptor.Enable()
		currentHeader, err = client.EthClient.HeaderByNumber(ctx, receipt.BlockNumber)
		client.Interceptor.Disable()
		if err != nil {
			return result, fmt.Errorf("fetching current header: (%T) %v", err, err)
		}
	}
	result.BlockTime = int64(currentHeader.Time)
	var baseFee uint64
	if currentHeader.BaseFee != nil {
		baseFee = currentHeader.BaseFee.Uint64()
	}

	latestHeader, err := client.EthClient.HeaderByNumber(ctx, nil)
	if err != nil {
		client.Interceptor.Enable()
		latestHeader, err = client.EthClient.HeaderByNumber(ctx, nil)
		client.Interceptor.Disable()
		if err != nil {
			return result, fmt.Errorf("fetching latest header: %v", err)
		}
	}
	result.Confirmations = latestHeader.Number.Int64() - receipt.BlockNumber.Int64()

	tokenMovements := tx.ParseTokenLogs(receipt, xc.NativeAsset(nativeAsset.Chain))
	// ethMovements, err := client.TraceEthMovements(ctx, txHash)
	var ethMovements tx.SourcesAndDests
	if os.Getenv("EVM_DEBUG_TRACE") == "1" {
		ethMovements, err = client.DebugTraceEthMovements(ctx, txHash)
	} else if os.Getenv("EVM_TRACE") == "1" {
		ethMovements, err = client.TraceEthMovements(ctx, txHash)
	} else {
		// default to debug trace as currently trace_transaction is missing eip7702 internal transfers.
		ethMovements, err = client.DebugTraceEthMovements(ctx, txHash)
		if err != nil {
			// fallback to trace_transaction if debug_traceTransaction fails
			ethMovements, err = client.TraceEthMovements(ctx, txHash)
		}
	}

	if err != nil {
		// Not all RPC nodes support this trace call, so we'll just drop reporting
		// internal eth movements if there's an issue.
		logrus.WithFields(logrus.Fields{
			"tx_hash": txHashStr,
			"chain":   nativeAsset.Chain,
			"error":   err,
		}).Warn("could not trace ETH tx")
		// set default eth movements
		amount := trans.Value.Int()
		zero := big.NewInt(0)

		from := trans.From
		to := ""
		if trans.ToMaybe != nil {
			to = trans.ToMaybe.String()
		}
		if from != "" && amount.Cmp(zero) > 0 {
			ethMovements = tx.SourcesAndDests{
				Sources: []*xclient.LegacyTxInfoEndpoint{{
					Address:     xc.Address(from),
					NativeAsset: nativeAsset.Chain,
					Amount:      xc.AmountBlockchain(*amount),
					Event:       xclient.NewEvent("", xclient.MovementVariantNative),
				}},
				Destinations: []*xclient.LegacyTxInfoEndpoint{{
					Address:     xc.Address(to),
					NativeAsset: nativeAsset.Chain,
					Amount:      xc.AmountBlockchain(*amount),
					Event:       xclient.NewEvent("", xclient.MovementVariantNative),
				}},
			}
		}
	}
	result.Fee = tx.Fee(trans.MaxPriorityFeePerGas, trans.GasPrice, baseFee, gasUsed)
	result.FeePayer = xc.Address(trans.From)
	result.Sources = append(ethMovements.Sources, tokenMovements.Sources...)
	result.Destinations = append(ethMovements.Destinations, tokenMovements.Destinations...)

	// Look for stake/unstake events
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}
		ev, _ := stake_deposit.EventByID(log.Topics[0])
		if ev != nil {
			// fmt.Println("found staking event")
			dep, err := stake_deposit.ParseDeposit(*log)
			if err != nil {
				logrus.WithError(err).Error("could not parse stake deposit log")
				continue
			}
			address := hex.EncodeToString(dep.WithdrawalCredentials)
			switch dep.WithdrawalCredentials[0] {
			case 1:
				// withdraw credential is an address
				address = hex.EncodeToString(dep.WithdrawalCredentials[len(dep.WithdrawalCredentials)-common.AddressLength:])
			}

			result.AddStakeEvent(&xclient.Stake{
				Balance:   dep.Amount,
				Validator: normalize.NormalizeAddressString(hex.EncodeToString(dep.Pubkey), nativeAsset.Chain),
				Address:   normalize.NormalizeAddressString(address, nativeAsset.Chain),
			})
		}
		ev, _ = exit_request.EventByID(log.Topics[0])
		if ev != nil {
			exitLog, err := exit_request.ParseExistRequest(*log)
			if err != nil {
				logrus.WithError(err).Error("could not parse stake exit log")
				continue
			}
			// assume 32 ether
			inc, _ := xc.NewAmountHumanReadableFromStr("32")
			result.AddStakeEvent(&xclient.Unstake{
				Balance:   inc.ToBlockchain(client.Asset.GetChain().Decimals),
				Validator: normalize.NormalizeAddressString(hex.EncodeToString(exitLog.Pubkey), nativeAsset.Chain),
				Address:   normalize.NormalizeAddressString(hex.EncodeToString(exitLog.Caller[:]), nativeAsset.Chain),
			})
		}
	}

	// map in the legacy fields
	if len(result.Sources) > 0 && len(result.Destinations) > 0 {
		result.From = result.Sources[0].Address
		result.To = result.Destinations[0].Address
		result.Amount = result.Destinations[0].Amount
		result.ContractAddress = result.Destinations[0].ContractAddress
		if len(result.Sources) > 1 && len(result.Destinations) > 1 {
			// legacy behavior..  map to evm `value` in case of multi source/dest
			result.Amount = trans.Value
		}
	} else {
		result.Amount = trans.Value
	}
	if result.From == "" {
		result.From = xc.Address(strings.ToLower(trans.From))
	}

	if result.Error != "" {
		// drop all changes
		result.Sources = nil
		result.Destinations = nil
		result.ResetStakeEvents()
	}
	// normalize
	for _, movement := range result.Sources {
		movement.Address = xc.Address(strings.ToLower(string(movement.Address)))
		movement.ContractAddress = xc.ContractAddress(strings.ToLower(string(movement.ContractAddress)))
		movement.ContractId = xc.ContractAddress(strings.ToLower(string(movement.ContractId)))
	}
	for _, movement := range result.Destinations {
		movement.Address = xc.Address(strings.ToLower(string(movement.Address)))
		movement.ContractAddress = xc.ContractAddress(strings.ToLower(string(movement.ContractAddress)))
		movement.ContractId = xc.ContractAddress(strings.ToLower(string(movement.ContractId)))
	}
	result.To = xc.Address(strings.ToLower(string(result.To)))
	result.From = xc.Address(strings.ToLower(string(result.From)))
	result.ContractAddress = xc.ContractAddress(strings.ToLower(string(result.ContractAddress)))

	return result, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, args *xclient.TxInfoArgs) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, args.TxHash())
	if err != nil {
		return xclient.TxInfo{}, err
	}
	chain := client.Asset.GetChain()

	// remap to new tx
	return xclient.TxInfoFromLegacy(chain, legacyTx, xclient.Account), nil
}

// Fetch the balance of the native asset that this client is configured for
func (client *Client) FetchNativeBalance(ctx context.Context, addr xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	targetAddr, err := address.FromHex(addr)
	if err != nil {
		return zero, fmt.Errorf("bad to address '%v': %v", addr, err)
	}
	balance, err := client.EthClient.BalanceAt(ctx, targetAddr, nil)
	if err != nil {
		return zero, fmt.Errorf("failed to get balance for '%v': %v", addr, err)
	}

	return xc.AmountBlockchain(*balance), nil
}

// Fetch the balance of the asset that this client is configured for
func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	if contract, ok := args.Contract(); ok {
		// token
		zero := xc.NewAmountBlockchainFromUint64(0)
		tokenAddress, _ := address.FromHex(xc.Address(contract))
		instance, err := erc20.NewErc20(tokenAddress, client.EthClient)
		if err != nil {
			return zero, err
		}

		dstAddress, _ := address.FromHex(args.Address())
		balance, err := instance.BalanceOf(&bind.CallOpts{}, dstAddress)
		if err != nil {
			return zero, err
		}
		return xc.AmountBlockchain(*balance), nil
	} else {

		// native
		return client.FetchNativeBalance(ctx, args.Address())
	}

}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}

	tokenAddress, _ := address.FromHex(xc.Address(contract))
	instance, err := erc20.NewErc20(tokenAddress, client.EthClient)
	if err != nil {
		return 0, err
	}
	dec, err := instance.Decimals(&bind.CallOpts{})
	if err != nil {
		return int(dec), err
	}
	return int(dec), nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	var err error
	height, ok := args.Height()
	if !ok {
		height, err = client.EthClient.BlockNumber(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get current block number: %v", err)
		}
	}
	bigHeight := big.NewInt(0)
	bigHeight.SetUint64(height)
	var evmBlock rpctypes.Block
	err = client.EthClient.Client().CallContext(ctx, &evmBlock, "eth_getBlockByNumber", "0x"+bigHeight.Text(16), false)
	if err != nil {
		return nil, fmt.Errorf("could not download block: %v", err)
	}

	returnedNumber := big.NewInt(0)
	_, ok = returnedNumber.SetString(evmBlock.Number, 0)
	if !ok {
		return nil, fmt.Errorf("could not parse downloaded block number: %s", evmBlock.Number)
	}
	unix := big.NewInt(0)
	_, ok = unix.SetString(evmBlock.Timestamp, 0)
	if !ok {
		return nil, fmt.Errorf("could not parse downloaded timestamp: %s", evmBlock.Timestamp)
	}

	block := &xclient.BlockWithTransactions{
		Block:          *xclient.NewBlock(client.Asset.GetChain().Chain, returnedNumber.Uint64(), evmBlock.Hash, time.Unix(int64(unix.Uint64()), 0)),
		TransactionIds: evmBlock.Transactions,
	}

	return block, nil
}
