package client

import (
	"context"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"
)

type TraceTransactionType string

var DELEGATE_CALL TraceTransactionType = "DELEGATECALL"
var CALL TraceTransactionType = "CALL"

type TraceTransactionResult struct {
	Gas     hexutil.Uint              `json:"gas"`
	GasUsed hexutil.Uint              `json:"gasUsed"`
	To      common.Address            `json:"to"`
	From    common.Address            `json:"from"`
	Input   hexutil.Bytes             `json:"input"`
	Value   hexutil.Big               `json:"value"`
	Type    TraceTransactionType      `json:"type"`
	Calls   []*TraceTransactionResult `json:"calls"`
}

type TraceTransactionArgs struct {
	Tracer string `json:"tracer"`
}

// Recurse through all of the traces and provide them as a linear set of traces.
func FlattenTraceResult(result *TraceTransactionResult, traces []*TraceTransactionResult) []*TraceTransactionResult {
	traces = append(traces, result)

	for _, innerResult := range result.Calls {
		traces = FlattenTraceResult(innerResult, traces)
	}
	return traces
}

// Implements debug_traceTransaction, which is supported on GETH and most RPC providers,
// but likely not implemented on public nodes.
// This will reveal ETH transfers in internal transactions and removes the need for us
// to manually parse "multi transfers".
func (client *Client) TraceTransaction(ctx context.Context, txHash common.Hash) (*TraceTransactionResult, error) {
	var result TraceTransactionResult
	err := client.EthClient.Client().CallContext(ctx, &result, "debug_traceTransaction", txHash, &TraceTransactionArgs{
		Tracer: "callTracer",
	})
	return &result, err
}

func (client *Client) TraceEthMovements(ctx context.Context, txHash common.Hash) (tx.SourcesAndDests, error) {
	result, err := client.TraceTransaction(ctx, txHash)
	if err != nil {
		return tx.SourcesAndDests{}, err
	}
	traces := FlattenTraceResult(result, []*TraceTransactionResult{})
	sourcesAndDests := tx.SourcesAndDests{}
	zero := big.NewInt(0)
	native := client.Asset.GetChain().Chain

	for _, trace := range traces {
		amount := trace.Value.ToInt()
		logrus.WithFields(logrus.Fields{
			"from":   trace.From.String(),
			"to":     trace.To.String(),
			"amount": amount.String(),
		}).Debug("trace")

		if amount.Cmp(zero) > 0 {
			amount := xc.AmountBlockchain(*trace.Value.ToInt())
			sourcesAndDests.Sources = append(sourcesAndDests.Sources, &xc.LegacyTxInfoEndpoint{
				Address:     xc.Address(trace.From.String()),
				Amount:      amount,
				NativeAsset: native,
			})
			sourcesAndDests.Destinations = append(sourcesAndDests.Destinations, &xc.LegacyTxInfoEndpoint{
				Address:     xc.Address(trace.To.String()),
				Amount:      amount,
				NativeAsset: native,
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
