package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"
)

// type TraceTransactionType string

// var DELEGATE_CALL TraceTransactionType = "DELEGATECALL"
// var CALL TraceTransactionType = "CALL"

type DebugTraceTransactionResult struct {
	Gas          hexutil.Uint                   `json:"gas"`
	GasUsed      hexutil.Uint                   `json:"gasUsed"`
	To           common.Address                 `json:"to"`
	From         common.Address                 `json:"from"`
	Input        hexutil.Bytes                  `json:"input"`
	Value        hexutil.Big                    `json:"value"`
	Type         TraceTransactionType           `json:"type"`
	Error        string                         `json:"error,omitempty"`
	RevertReason string                         `json:"revertReason,omitempty"`
	Calls        []*DebugTraceTransactionResult `json:"calls"`
	traceId      string
}

type DebugTraceTransactionArgs struct {
	Tracer string `json:"tracer"`
}

// Recurse through all of the traces and provide them as a linear set of traces.
func FlattenTraceResult(result *DebugTraceTransactionResult, traces []*DebugTraceTransactionResult, id string) []*DebugTraceTransactionResult {
	traces = append(traces, result)
	result.traceId = strings.TrimPrefix(id, "_")
	index := 0

	for _, innerResult := range result.Calls {
		// We want the events to be consistent between `trace_transaction` and `debug_traceTransaction`.
		// However, `trace_transaction` seems to omit `STATICCALL`'s to internal contracts.
		// The only way to tell if it's an internal contract is if it has an impossible amount of 0's in the address.
		if innerResult.Type == STATIC_CALL {
			// If 16/20 or more of the bytes are 0, we'll assume it's an internal contract.
			if bytes.Count(innerResult.To[:], []byte{0x00}) > 15 {
				continue
			}
		}
		traces = FlattenTraceResult(innerResult, traces, fmt.Sprintf("%s_%d", id, index))
		index++
	}
	return traces
}

// Implements debug_traceTransaction, which is supported on GETH and most RPC providers,
// but likely not implemented on public nodes.
// This will reveal ETH transfers in internal transactions and removes the need for us
// to manually parse "multi transfers".
func (client *Client) DebugTraceTransaction(ctx context.Context, txHash common.Hash) (*DebugTraceTransactionResult, error) {
	var result DebugTraceTransactionResult
	err := client.EthClient.Client().CallContext(ctx, &result, "debug_traceTransaction", txHash, &DebugTraceTransactionArgs{
		Tracer: "callTracer",
	})
	return &result, err
}

func printJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}

func (client *Client) DebugTraceEthMovements(ctx context.Context, txHash common.Hash) (tx.SourcesAndDests, error) {
	result, err := client.DebugTraceTransaction(ctx, txHash)
	if err != nil {
		return tx.SourcesAndDests{}, err
	}
	traces := FlattenTraceResult(result, []*DebugTraceTransactionResult{}, "")
	sourcesAndDests := tx.SourcesAndDests{}
	zero := big.NewInt(0)
	native := client.Asset.GetChain().Chain

	if len(traces) == 0 {
		logrus.Debug("no debug traces found for tx")
	}
	printJson(result)

	for _, trace := range traces {
		amount := trace.Value.ToInt()
		logrus.WithFields(logrus.Fields{
			"from":   trace.From.String(),
			"to":     trace.To.String(),
			"amount": amount.String(),
			"error":  trace.Error,
		}).Debug("trace")
		if trace.Error != "" || trace.RevertReason != "" {
			// stop tracing if we hit a reverted instruction, so we remain on the side of caution.
			break
		}
		if amount.Cmp(zero) <= 0 {
			continue
		}

		xcAmount := xc.AmountBlockchain(*trace.Value.ToInt())
		event := txinfo.NewEvent(trace.traceId, txinfo.MovementVariantInternal)
		if trace.traceId == "" {
			// this is just the native eth transfer (.value in the tx).
			event = txinfo.NewEvent("", txinfo.MovementVariantNative)
		}
		sourcesAndDests.Sources = append(sourcesAndDests.Sources, &txinfo.LegacyTxInfoEndpoint{
			Event:       event,
			Address:     xc.Address(trace.From.String()),
			Amount:      xcAmount,
			NativeAsset: native,
		})
		sourcesAndDests.Destinations = append(sourcesAndDests.Destinations, &txinfo.LegacyTxInfoEndpoint{
			Event:       event,
			Address:     xc.Address(trace.To.String()),
			Amount:      xcAmount,
			NativeAsset: native,
		})
	}

	return sourcesAndDests, nil

}
