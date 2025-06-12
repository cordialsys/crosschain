package client

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"
)

type TraceTransactionType string

var DELEGATE_CALL TraceTransactionType = "DELEGATECALL"
var STATIC_CALL TraceTransactionType = "STATICCALL"
var CALL TraceTransactionType = "CALL"

// TraceAction represents a single action in a transaction trace
type TraceAction struct {
	From     common.Address `json:"from"`
	To       common.Address `json:"to"`
	Value    hexutil.Big    `json:"value"`
	Gas      hexutil.Big    `json:"gas"`
	Input    hexutil.Bytes  `json:"input"`
	CallType string         `json:"callType"`
}

// TraceResult represents the result of a single trace action
type TraceResult struct {
	GasUsed hexutil.Uint  `json:"gasUsed"`
	Output  hexutil.Bytes `json:"output"`
}

// TraceTransactionEntry represents a single entry in a transaction trace
type TraceTransactionEntry struct {
	Action              TraceAction `json:"action"`
	Result              TraceResult `json:"result"`
	Subtraces           int         `json:"subtraces"`
	TraceAddress        []int       `json:"traceAddress"`
	TransactionHash     string      `json:"transactionHash"`
	TransactionPosition int         `json:"transactionPosition"`
	Type                string      `json:"type"`
}

// TraceTransactionResult represents the result of a transaction trace
type TraceTransactionResult []TraceTransactionEntry

// Implements trace_transaction, which is supported on most RPC providers.
// This will reveal ETH transfers in internal transactions and removes the need for us
// to manually parse "multi transfers".
func (client *Client) TraceTransaction(ctx context.Context, txHash common.Hash) (TraceTransactionResult, error) {
	var result TraceTransactionResult
	// var jsonResult json.RawMessage
	err := client.EthClient.Client().CallContext(ctx, &result, "trace_transaction", txHash)
	if err != nil {
		return nil, err
	}
	// fmt.Println("JSON RESULT", string(jsonResult))
	// err = json.Unmarshal(jsonResult, &result)
	// if err != nil {
	// 	return nil, err
	// }
	return result, err
}

func (client *Client) TraceEthMovements(ctx context.Context, txHash common.Hash) (tx.SourcesAndDests, error) {
	traces, err := client.TraceTransaction(ctx, txHash)
	if err != nil {
		return tx.SourcesAndDests{}, err
	}

	sourcesAndDests := tx.SourcesAndDests{}
	zero := big.NewInt(0)
	native := client.Asset.GetChain().Chain

	for _, trace := range traces {
		amount := trace.Action.Value.ToInt()
		logrus.WithFields(logrus.Fields{
			"from":   trace.Action.From.String(),
			"to":     trace.Action.To.String(),
			"amount": amount.String(),
		}).Debug("trace")

		// The trace is identified by the trace address, which is oddly an array of ints.
		// I believe this is because the evm contract calls can be a dynamically nested structure,
		// So the array is to preserve the path taken to get to the current trace.
		// We join using a '_' as this is a common normalization, and it is also what alchemy does.
		traceAddressParts := make([]string, len(trace.TraceAddress))
		for i, part := range trace.TraceAddress {
			traceAddressParts[i] = strconv.Itoa(part)
		}
		traceId := strings.Join(traceAddressParts, "_")

		eventMeta := xclient.NewEvent(traceId, xclient.MovementVariantInternal)
		if traceId == "" {
			// this is just the native eth transfer (.value in the tx).
			eventMeta = xclient.NewEvent("", xclient.MovementVariantNative)
		}
		if amount.Cmp(zero) > 0 {
			amount := xc.AmountBlockchain(*trace.Action.Value.ToInt())
			sourcesAndDests.Sources = append(sourcesAndDests.Sources, &xclient.LegacyTxInfoEndpoint{
				Address:     xc.Address(trace.Action.From.String()),
				Amount:      amount,
				NativeAsset: native,
				Event:       eventMeta,
			})
			sourcesAndDests.Destinations = append(sourcesAndDests.Destinations, &xclient.LegacyTxInfoEndpoint{
				Address:     xc.Address(trace.Action.To.String()),
				Amount:      amount,
				NativeAsset: native,
				Event:       eventMeta,
			})
		}
	}

	return sourcesAndDests, nil
}

type TxPoolResult struct {
	// map of nonce to txinfo
	Pending map[string]*TxPoolTxInfo `json:"pending"`
	// Queued  map[string]*TxPoolTxInfo `json:"pending"`
}

func (result *TxPoolResult) PendingCount() int {
	if result.Pending == nil {
		return 0
	}
	return len(result.Pending)
}

// return first pending tx for a given address (should only be 1 entry..)
func (result *TxPoolResult) InfoFor(address string) (*TxPoolTxInfo, bool) {
	for _, info := range result.Pending {
		if strings.EqualFold(info.From, address) {
			return info, true
		}
	}
	return nil, false
}

type TxPoolPendingMap struct {
}
type TxPoolTxInfo struct {
	From                 string      `json:"from"`
	Gas                  hexutil.Big `json:"gas"`
	GasPrice             hexutil.Big `json:"gasPrice"`
	MaxFeePerGas         hexutil.Big `json:"maxFeePerGas"`
	MaxPriorityFeePerGas hexutil.Big `json:"maxPriorityFeePerGas"`
	Hash                 string      `json:"hash"`
}

// Get current pending transaction queue for a given address
func (client *Client) TxPoolContentFrom(ctx context.Context, from common.Address) (*TxPoolResult, error) {
	var result TxPoolResult
	err := client.EthClient.Client().CallContext(ctx, &result, "txpool_contentFrom", from.Hex())
	return &result, err
}
