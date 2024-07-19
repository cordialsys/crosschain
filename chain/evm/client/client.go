package client

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/erc20"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/utils"
	"github.com/ethereum/go-ethereum"
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

// ChainID returns the ChainID
// func (client *Client) ChainID() (*big.Int, error) {
// 	var err error
// 	if client.ChainId == nil {
// 		client.ChainId, err = client.EthClient.ChainID(context.Background())
// 	}
// 	return client.ChainId, err
// }

func (client *Client) DefaultMaxGasLimit() uint64 {
	// Set absolute gas limits for safety
	gasLimit := uint64(90_000)
	native := client.Asset.GetChain()
	if client.Asset.GetContract() != "" {
		// token
		gasLimit = 500_000
	}
	if native.Chain == xc.ArbETH {
		// arbeth specifically has different gas limit scale
		gasLimit = 4_000_000
	}
	return gasLimit
}

// Simulate a transaction to get the estimated gas limit
func (client *Client) SimulateGasWithLimit(ctx context.Context, txBuilder xc.TxBuilder, from xc.Address, to xc.Address, txInput xc.TxInput) (uint64, error) {
	zero := big.NewInt(0)
	fromAddr, _ := address.FromHex(from)

	// TODO it may be more accurate to use the actual amount for the transfer,
	// but that will require changing the interface to pass amount.
	// For now we'll use the smallest amount (1).
	trans, err := txBuilder.NewTransfer(from, to, xc.NewAmountBlockchainFromUint64(1), txInput)
	if err != nil {
		return 0, fmt.Errorf("could not build simulated tx: %v", err)
	}
	ethTx := trans.(*tx.Tx).EthTx
	msg := ethereum.CallMsg{
		From: fromAddr,
		To:   ethTx.To(),
		// use a high limit just for the estimation
		Gas:        8_000_000,
		GasPrice:   zero,
		GasFeeCap:  zero,
		GasTipCap:  zero,
		Value:      ethTx.Value(),
		Data:       ethTx.Data(),
		AccessList: types.AccessList{},
	}
	gasLimit, err := client.EthClient.EstimateGas(ctx, msg)
	if err != nil && strings.Contains(err.Error(), "insufficient funds") {
		// try getting gas estimate without sending funds
		msg.Value = zero
		gasLimit, err = client.EthClient.EstimateGas(ctx, msg)
	} else if err != nil && strings.Contains(err.Error(), "less than the block's baseFeePerGas") {
		// this estimate does not work with hardhat -> use defaults
		return client.DefaultMaxGasLimit(), nil
	}
	if err != nil {
		return 0, fmt.Errorf("could not simulate tx: %v", err)
	}

	// heuristic: Sometimes tokens can have inconsistent gas spends. Where the gas spent is _sometimes_ higher than what we see in simulation.
	// To avoid this, we can opportunistically increase the gas budget if there is Enough native asset present.  We don't want to increase the gas budget if we can't
	// afford it, as this can also be a source of failure.
	if client.Asset.GetContract() != "" {
		// always add 1k gas extra
		gasLimit += 1_000
		amountEth, err := client.FetchNativeBalance(ctx, from)
		oneEthHuman, _ := xc.NewAmountHumanReadableFromStr("1")
		oneEth := oneEthHuman.ToBlockchain(client.Asset.GetChain().Decimals)
		// add 70k more if we can clearly afford it
		if err == nil && amountEth.Cmp(&oneEth) >= 0 {
			// increase gas budget 70k
			gasLimit += 70_000
		}
	}

	defaultMax := client.DefaultMaxGasLimit()
	if gasLimit == 0 || gasLimit > defaultMax {
		gasLimit = defaultMax
	}
	return gasLimit, nil
}

func (client *Client) GetNonce(ctx context.Context, from xc.Address) (uint64, error) {
	var fromAddr common.Address
	var err error
	fromAddr, err = address.FromHex(from)
	if err != nil {
		return 0, fmt.Errorf("bad to address '%v': %v", from, err)
	}
	nonce, err := client.EthClient.NonceAt(ctx, fromAddr, nil)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

// FetchTxInput returns tx input for a EVM tx
func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	nativeAsset := client.Asset.GetChain()

	zero := xc.NewAmountBlockchainFromUint64(0)
	result := tx_input.NewTxInput()

	// Gas tip (priority fee) calculation
	result.GasTipCap = xc.NewAmountBlockchainFromUint64(DEFAULT_GAS_TIP)
	result.GasFeeCap = zero

	// Nonce
	nonce, err := client.GetNonce(ctx, from)
	if err != nil {
		return result, err
	}
	result.Nonce = nonce

	// chain ID
	chainId, err := client.EthClient.ChainID(ctx)
	if err != nil {
		return result, fmt.Errorf("could not lookup chain_id: %v", err)
	}
	result.ChainId = xc.AmountBlockchain(*chainId)

	// Gas
	if !nativeAsset.NoGasFees {
		latestHeader, err := client.EthClient.HeaderByNumber(ctx, nil)
		if err != nil {
			return result, err
		}

		gasTipCap, err := client.EthClient.SuggestGasTipCap(ctx)
		if err != nil {
			return result, err
		}
		result.GasFeeCap = xc.AmountBlockchain(*latestHeader.BaseFee)
		// should only multiply one cap, not both.
		result.GasTipCap = xc.AmountBlockchain(*gasTipCap).ApplyGasPriceMultiplier(client.Asset.GetChain())

		if result.GasFeeCap.Cmp(&result.GasTipCap) < 0 {
			// increase max fee cap to accomodate tip if needed
			result.GasFeeCap = result.GasTipCap
		}

		fromAddr, _ := address.FromHex(from)
		pendingTxInfo, err := client.TxPoolContentFrom(ctx, fromAddr)
		if err != nil {
			logrus.WithFields(logrus.Fields{"from": from, "err": err}).Warn("could not see pending tx pool")
		} else {
			pending, ok := pendingTxInfo.InfoFor(string(from))
			if ok {
				// if there's a pending tx, we want to replace it (use 15% increase).
				minMaxFee := xc.MultiplyByFloat(xc.AmountBlockchain(*pending.MaxFeePerGas.ToInt()), 1.15)
				minPriorityFee := xc.MultiplyByFloat(xc.AmountBlockchain(*pending.MaxPriorityFeePerGas.ToInt()), 1.15)
				log := logrus.WithFields(logrus.Fields{
					"from":        from,
					"old-tx":      pending.Hash,
					"old-fee-cap": result.GasFeeCap.String(),
					"new-fee-cap": minMaxFee.String(),
				})
				if result.GasFeeCap.Cmp(&minMaxFee) < 0 {
					log.Debug("replacing max-fee-cap because of pending tx")
					result.GasFeeCap = minMaxFee
				}
				if result.GasTipCap.Cmp(&minPriorityFee) < 0 {
					log.Debug("replacing max-priority-fee-cap because of pending tx")
					result.GasTipCap = minPriorityFee
				}
			}
		}

	} else {
		result.GasTipCap = zero
	}

	builder, err := builder.NewTxBuilder(client.Asset)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	gasLimit, err := client.SimulateGasWithLimit(ctx, builder, from, to, result)
	if err != nil {
		return nil, err
	}
	result.GasLimit = uint64(gasLimit)

	return result, nil
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

	// reverted tx
	result.BlockIndex = receipt.BlockNumber.Int64()
	result.BlockHash = receipt.BlockHash.Hex()
	gasUsed := receipt.GasUsed
	if receipt.Status == 0 {
		result.Status = xc.TxStatusFailure
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
