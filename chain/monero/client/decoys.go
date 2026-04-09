package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"crypto/rand"

	"github.com/sirupsen/logrus"
)

const (
	// ringSize is the number of ring members per input (1 real + 15 decoys)
	ringSize = 16
)

// DecoyOutput represents a decoy output fetched from the blockchain
type DecoyOutput struct {
	GlobalIndex uint64
	PublicKey   string // hex
	Commitment  string // hex (rct commitment)
}

// FetchDecoys selects decoy ring members for a transaction input.
// It picks random outputs from the blockchain distribution, avoiding the real output.
func (c *Client) FetchDecoys(ctx context.Context, realGlobalIndex uint64, count int) ([]DecoyOutput, error) {
	// Get total output count from get_info (lightweight, no huge distribution array)
	infoResult, err := c.jsonRPCRequest(ctx, "get_info", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get info: %w", err)
	}
	var info struct {
		TxCount uint64 `json:"tx_count"`
		Height  uint64 `json:"height"`
	}
	if err := json.Unmarshal(infoResult, &info); err != nil {
		return nil, fmt.Errorf("failed to parse info: %w", err)
	}

	// Use the real output's global index as the upper bound.
	// Decoys should be from outputs that exist (index <= our output's index).
	totalOutputs := realGlobalIndex
	if totalOutputs < uint64(count+1) {
		// On a very new chain, use tx count estimate
		totalOutputs = info.TxCount * 2
	}
	if totalOutputs < uint64(count+1) {
		return nil, fmt.Errorf("not enough outputs on chain for ring size %d", count)
	}

	// Select random global indices using gamma distribution (Monero's approach)
	// Simplified: uniform random selection weighted toward recent outputs
	selectedIndices := selectDecoyIndices(totalOutputs, realGlobalIndex, count)

	// Fetch the output data for selected indices
	outs, err := c.fetchOutputs(ctx, selectedIndices)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch decoy outputs: %w", err)
	}

	return outs, nil
}

// selectDecoyIndices picks random output indices, avoiding the real output.
// Uses a simplified version of Monero's gamma distribution for recent-output bias.
func selectDecoyIndices(totalOutputs uint64, realIndex uint64, count int) []uint64 {
	selected := make(map[uint64]bool)
	selected[realIndex] = true // avoid picking the real output as decoy

	indices := make([]uint64, 0, count)
	maxAttempts := count * 20

	for len(indices) < count && maxAttempts > 0 {
		maxAttempts--

		// Gamma-like distribution: bias toward recent outputs
		// Use rejection sampling with a simple triangular distribution
		randBytes := make([]byte, 8)
		rand.Read(randBytes)
		r := new(big.Int).SetBytes(randBytes)
		r.Mod(r, new(big.Int).SetUint64(totalOutputs))
		idx := r.Uint64()

		// Bias toward recent: with 50% chance, pick from last 25% of outputs
		coin := make([]byte, 1)
		rand.Read(coin)
		if coin[0] < 128 && totalOutputs > 100 {
			recentStart := totalOutputs - totalOutputs/4
			rand.Read(randBytes)
			r2 := new(big.Int).SetBytes(randBytes)
			r2.Mod(r2, new(big.Int).SetUint64(totalOutputs/4))
			idx = recentStart + r2.Uint64()
		}

		if idx == 0 {
			idx = 1
		}
		if idx >= totalOutputs {
			idx = totalOutputs - 1
		}

		if !selected[idx] {
			selected[idx] = true
			indices = append(indices, idx)
		}
	}

	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	return indices
}

// fetchOutputs retrieves output data (public key and commitment) for given global indices.
func (c *Client) fetchOutputs(ctx context.Context, indices []uint64) ([]DecoyOutput, error) {
	getOuts := make([]map[string]uint64, len(indices))
	for i, idx := range indices {
		getOuts[i] = map[string]uint64{"amount": 0, "index": idx}
	}

	result, err := c.httpRequest(ctx, "/get_outs", map[string]interface{}{
		"outputs":   getOuts,
		"get_txid":  false,
	})
	if err != nil {
		return nil, fmt.Errorf("get_outs failed: %w", err)
	}

	var outsResp struct {
		Outs []struct {
			Key    string `json:"key"`
			Mask   string `json:"mask"`
			Txid   string `json:"txid"`
			Height uint64 `json:"height"`
		} `json:"outs"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &outsResp); err != nil {
		return nil, fmt.Errorf("failed to parse get_outs response: %w", err)
	}
	if outsResp.Status != "OK" {
		return nil, fmt.Errorf("get_outs returned status: %s", outsResp.Status)
	}

	decoys := make([]DecoyOutput, len(outsResp.Outs))
	for i, out := range outsResp.Outs {
		decoys[i] = DecoyOutput{
			GlobalIndex: indices[i],
			PublicKey:   out.Key,
			Commitment:  out.Mask,
		}
	}

	logrus.WithField("count", len(decoys)).Debug("fetched decoy outputs")
	return decoys, nil
}

// BuildRing constructs a sorted ring of outputs for CLSAG signing.
// Returns the ring (sorted by global index), the position of the real output, and relative key offsets.
// See also: builder.buildRingFromMembers which does the same for tx_input.RingMember types.
func BuildRing(realIndex uint64, realKey string, realCommitment string, decoys []DecoyOutput) (ring []DecoyOutput, realPos int, keyOffsets []uint64) {
	// Combine real output with decoys
	all := make([]DecoyOutput, 0, len(decoys)+1)
	all = append(all, DecoyOutput{
		GlobalIndex: realIndex,
		PublicKey:   realKey,
		Commitment:  realCommitment,
	})
	all = append(all, decoys...)

	// Sort by global index
	sort.Slice(all, func(i, j int) bool { return all[i].GlobalIndex < all[j].GlobalIndex })

	// Find real output position after sorting
	realPos = -1
	for i, out := range all {
		if out.GlobalIndex == realIndex {
			realPos = i
			break
		}
	}

	// Compute relative key offsets (each offset is relative to the previous)
	keyOffsets = make([]uint64, len(all))
	var prev uint64
	for i, out := range all {
		keyOffsets[i] = out.GlobalIndex - prev
		prev = out.GlobalIndex
	}

	return all, realPos, keyOffsets
}
