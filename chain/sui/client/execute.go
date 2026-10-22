package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	xctypes "github.com/cordialsys/crosschain/client/types"
)

func (c *Client) simulateTransactionGasFee(ctx context.Context, transaction xc.Tx, isNative bool) (uint64, bool, error) {
	serialized, err := transaction.Serialize()
	if err != nil {
		return 0, false, err
	}
	// Simulation masks are relative to SimulateTransactionResponse, unlike execution.
	resp, err := c.Execution.SimulateTransaction(ctx, &pb.SimulateTransactionRequest{
		Transaction: &pb.Transaction{Bcs: &pb.Bcs{Value: serialized}},
		ReadMask:    mask("transaction.effects.status", "transaction.effects.gas_used"),
	})
	if err != nil {
		return 0, false, fmt.Errorf("simulate Sui transaction: %w", err)
	}
	effects := resp.GetTransaction().GetEffects()
	if effects.GetStatus() == nil || effects.Status.Success == nil {
		return 0, false, fmt.Errorf("Sui simulation returned no execution status")
	}
	// Match the existing client's behavior: a failed simulation leaves the
	// conservative input budget intact (e.g. while estimating an unfunded tx).
	if !effects.Status.GetSuccess() {
		return 0, false, nil
	}
	gas := effects.GetGasUsed()
	if gas == nil || gas.ComputationCost == nil || gas.StorageCost == nil || gas.StorageRebate == nil {
		return 0, false, fmt.Errorf("Sui simulation returned incomplete gas costs")
	}
	if gas.GetComputationCost() > math.MaxUint64-gas.GetStorageCost() {
		return 0, false, fmt.Errorf("Sui simulation gas cost overflow")
	}
	fee := gas.GetComputationCost() + gas.GetStorageCost()
	if gas.GetStorageRebate() > fee {
		fee = 0
	} else {
		fee -= gas.GetStorageRebate()
	}
	minimum := c.Asset.GasBudgetMinimum.ToBlockchain(c.Asset.Decimals).Uint64()
	if minimum == 0 {
		minimum = 2_000_000
	}
	if fee < minimum {
		fee = minimum
	}
	if !isNative {
		// Round up the token margin without overflowing an intermediate product.
		margin := fee / 10
		if fee%10 != 0 {
			margin++
		}
		if fee > math.MaxUint64-margin {
			return 0, false, fmt.Errorf("Sui gas budget overflow")
		}
		fee += margin
	}
	return fee, true, nil
}

// SubmitTx sends the existing TransactionData BCS and scheme/signature/public-key
// bytes unchanged. Both the metadata and legacy signatures formats are accepted.
func (c *Client) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq, _ builder.SubmitArgs) error {
	serialized, err := tx.Serialize()
	if err != nil {
		return err
	}
	data, ok, err := tx.GetMetadata()
	if err != nil {
		return err
	}
	var metadata sui.BroadcastMetadata
	if ok {
		if err := json.Unmarshal(data, &metadata); err != nil {
			return fmt.Errorf("decode Sui broadcast metadata: %w", err)
		}
	}
	signatures := metadata.Signatures
	if len(signatures) == 0 {
		for _, signature := range tx.GetSignatures() {
			signatures = append(signatures, []byte(signature))
		}
	}
	if len(signatures) == 0 {
		return fmt.Errorf("Sui transaction has no signatures")
	}
	// Execution masks are relative to ExecutedTransaction (without transaction.).
	req := &pb.ExecuteTransactionRequest{Transaction: &pb.Transaction{Bcs: &pb.Bcs{Value: serialized}}, ReadMask: mask("effects.status")}
	for _, signature := range signatures {
		req.Signatures = append(req.Signatures, &pb.UserSignature{Bcs: &pb.Bcs{Value: signature}})
	}
	resp, err := c.Execution.ExecuteTransaction(ctx, req)
	if err != nil {
		return err
	}
	status := resp.GetTransaction().GetEffects().GetStatus()
	if status == nil || status.Success == nil {
		return fmt.Errorf("Sui execution returned no status")
	}
	if !status.GetSuccess() {
		return fmt.Errorf("Sui transaction failed: %s", executionError(status))
	}
	return nil
}

func executionError(status *pb.ExecutionStatus) string {
	if description := status.GetError().GetDescription(); description != "" {
		return description
	}
	return status.GetError().GetKind().String()
}
