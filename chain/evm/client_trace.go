package evm

import (
	"context"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

func (client *Client) TraceEthMovements(ctx context.Context, txHash common.Hash) (SourcesAndDests, error) {

	result, err := client.TraceTransaction(ctx, txHash)
	if err != nil {
		return SourcesAndDests{}, err
	}
	traces := FlattenTraceResult(result, []*TraceTransactionResult{})
	sourcesAndDests := SourcesAndDests{}
	zero := big.NewInt(0)
	native := client.Asset.GetChain().Chain

	for _, trace := range traces {
		if trace.Value.ToInt().Cmp(zero) > 0 {
			amount := xc.AmountBlockchain(*trace.Value.ToInt())
			sourcesAndDests.Sources = append(sourcesAndDests.Destinations, &xc.LegacyTxInfoEndpoint{
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
