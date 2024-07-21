package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/erc20"
	"github.com/cordialsys/crosschain/chain/evm/abi/stake_deposit"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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

var _ xclient.FullClient = &Client{}
var _ xclient.ClientV2 = &Client{}

func configToEVMClientURL(cfgI xc.ITask) string {
	cfg := cfgI.GetChain()
	if cfg.Provider == "infura" {
		return cfg.URL + "/" + cfg.AuthSecret
	}
	return cfg.URL
}

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
	url := configToEVMClientURL(asset)

	// c, err := rpc.DialContext(context.Background(), url)
	interceptor := utils.NewHttpInterceptor(ReplaceIncompatiableEvmResponses)
	// {http.DefaultTransport, false}
	httpClient := &http.Client{
		Transport: interceptor,
	}
	c, err := rpc.DialHTTPWithClient(url, httpClient)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("dialing url: %v", nativeAsset.URL))
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
func (client *Client) SubmitTx(ctx context.Context, trans xc.Tx) error {
	switch tx := trans.(type) {
	case *tx.Tx:
		err := client.EthClient.SendTransaction(ctx, tx.EthTx)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("sending transaction '%v': %v", tx.Hash(), err))
		}
		return nil
	default:
		bz, err := tx.Serialize()
		if err != nil {
			return err
		}
		return client.EthClient.Client().CallContext(ctx, nil, "eth_sendRawTransaction", hexutil.Encode(bz))
	}
}

// FetchLegacyTxInfo returns tx info for a EVM tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHashStr xc.TxHash) (xc.LegacyTxInfo, error) {
	nativeAsset := client.Asset.GetChain()
	txHashHex := address.TrimPrefixes(string(txHashStr))
	txHash := common.HexToHash(txHashHex)

	result := xc.LegacyTxInfo{
		TxID:        txHashHex,
		ExplorerURL: nativeAsset.ExplorerURL + "/tx/0x" + txHashHex,
	}

	trans, pending, err := client.EthClient.TransactionByHash(ctx, txHash)
	if err != nil {
		// TODO retry only for KLAY
		client.Interceptor.Enable()
		trans, pending, err = client.EthClient.TransactionByHash(ctx, txHash)
		client.Interceptor.Disable()
		if err != nil {
			return result, fmt.Errorf(fmt.Sprintf("fetching tx by hash '%s': %v", txHashStr, err))
		}
	}

	chainID := new(big.Int).SetInt64(nativeAsset.ChainID)

	// If the transaction is still pending, return an empty txInfo.
	if pending {
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

	// // tx confirmed
	confirmedTx := tx.Tx{
		EthTx:  trans,
		Signer: types.LatestSignerForChainID(chainID),
	}

	tokenMovements := confirmedTx.ParseTokenLogs(receipt, xc.NativeAsset(nativeAsset.Chain))
	ethMovements, err := client.TraceEthMovements(ctx, txHash)
	if err != nil {
		// Not all RPC nodes support this trace call, so we'll just drop reporting
		// internal eth movements if there's an issue.
		logrus.WithFields(logrus.Fields{
			"tx_hash": txHashStr,
			"chain":   nativeAsset.Chain,
			"error":   err,
		}).Warn("could not trace ETH tx")
		// set default eth movements
		amount := trans.Value()
		zero := big.NewInt(0)
		if amount.Cmp(zero) > 0 {
			ethMovements = tx.SourcesAndDests{
				Sources: []*xc.LegacyTxInfoEndpoint{{
					Address:     confirmedTx.From(),
					NativeAsset: nativeAsset.Chain,
					Amount:      xc.AmountBlockchain(*amount),
				}},
				Destinations: []*xc.LegacyTxInfoEndpoint{{
					Address:     confirmedTx.To(),
					NativeAsset: nativeAsset.Chain,
					Amount:      xc.AmountBlockchain(*amount),
				}},
			}
		}
	}

	result.From = confirmedTx.From()
	result.To = confirmedTx.To()
	result.ContractAddress = confirmedTx.ContractAddress()
	result.Amount = confirmedTx.Amount()
	result.Fee = confirmedTx.Fee(baseFee, gasUsed)
	result.Sources = append(ethMovements.Sources, tokenMovements.Sources...)
	result.Destinations = append(ethMovements.Destinations, tokenMovements.Destinations...)

	for _, log := range receipt.Logs {
		ev, _ := stake_deposit.EventByID(log.Topics[0])
		if ev != nil {
			// fmt.Println("found staking event")
			dep, err := stake_deposit.ParseDeposit(*log)
			if err != nil {
				logrus.WithError(err).Error("could not parse stake deposit log")
				continue
			}
			// fmt.Println("stake event: ", dep.Amount, hex.EncodeToString(dep.Pubkey), hex.EncodeToString(dep.WithdrawalCredentials))
			result.AddStakeEvent(&xclient.Stake{
				Amount:             dep.Amount,
				Validator:          hex.EncodeToString(dep.Pubkey),
				WithdrawCredential: hex.EncodeToString(dep.WithdrawalCredentials),
			})
		}
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
func (client *Client) FetchBalance(ctx context.Context, addr xc.Address) (xc.AmountBlockchain, error) {
	// native
	if _, ok := client.Asset.(*xc.ChainConfig); ok {
		return client.FetchNativeBalance(ctx, addr)
	}

	// token
	contract := client.Asset.GetContract()
	zero := xc.NewAmountBlockchainFromUint64(0)
	tokenAddress, _ := address.FromHex(xc.Address(contract))
	instance, err := erc20.NewErc20(tokenAddress, client.EthClient)
	if err != nil {
		return zero, err
	}

	dstAddress, _ := address.FromHex(addr)
	balance, err := instance.BalanceOf(&bind.CallOpts{}, dstAddress)
	if err != nil {
		return zero, err
	}
	return xc.AmountBlockchain(*balance), nil
}
