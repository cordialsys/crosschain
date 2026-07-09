package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"

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
	Unlocked    bool
}

// FetchDecoys selects decoy ring members for a transaction input.
// It picks random outputs from the blockchain distribution, avoiding the real
// output and any locked outputs (e.g. coinbase outputs inside the 60-block
// unlock window) — a ring containing a locked member is rejected by the
// daemon with invalid_input.
func (c *Client) FetchDecoys(ctx context.Context, realGlobalIndex uint64, count int) ([]DecoyOutput, error) {
	// Decoys are drawn from the chain's entire RCT output distribution, not
	// capped at the real output's index: decoys newer than the real output are
	// valid ring members, and the daemon's relay sanity check rejects a tx
	// whose median ring-member index is below 60% of all outputs — an old real
	// output with same-age decoys fails it (and is terrible for privacy).
	totalOutputs, err := c.getRctOutputCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rct output count: %w", err)
	}
	if totalOutputs < uint64(count+1) {
		return nil, fmt.Errorf("not enough outputs on chain for ring size %d", count)
	}

	// monerod rejects a ring whose median member index is below 60% of all
	// outputs ("Sanity check failed"). The recency-biased draw clears this the
	// vast majority of the time; on the rare ring that falls short we re-draw the
	// WHOLE decoy set rather than editing it. Surgically replacing the lowest
	// decoys (the old approach) strips the very members that camouflage an older
	// real output, leaving it a conspicuous low outlier — a re-roll keeps every
	// ring an unbiased sample. Target 63% for margin over the daemon's slightly
	// larger current total.
	const (
		maxRingAttempts = 20
		targetPct       = 63
	)
	target := totalOutputs * targetPct / 100
	for attempt := 0; attempt < maxRingAttempts; attempt++ {
		decoys, err := c.drawDecoys(ctx, realGlobalIndex, count, totalOutputs)
		if err != nil {
			return nil, err
		}
		if len(decoys) == count && ringMedian(decoys, realGlobalIndex) >= target {
			return decoys, nil
		}
	}
	return nil, fmt.Errorf("could not build a ring meeting the median sanity threshold (>= %d) after %d attempts", target, maxRingAttempts)
}

// drawDecoys draws `count` unlocked decoy outputs (excluding the real output)
// from the recency-biased distribution, re-drawing to replace any locked
// outputs it hits. It returns fewer than `count` only when the chain cannot
// supply enough unlocked outputs.
func (c *Client) drawDecoys(ctx context.Context, realGlobalIndex uint64, count int, totalOutputs uint64) ([]DecoyOutput, error) {
	selected := map[uint64]bool{realGlobalIndex: true}
	decoys := make([]DecoyOutput, 0, count)
	const maxRounds = 10
	for round := 0; len(decoys) < count && round < maxRounds; round++ {
		indices := selectDecoyIndices(totalOutputs, selected, count-len(decoys))
		if len(indices) == 0 {
			break
		}
		outs, err := c.fetchOutputs(ctx, indices)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch decoy outputs: %w", err)
		}
		for _, out := range outs {
			if !out.Unlocked {
				logrus.WithField("global_index", out.GlobalIndex).Debug("skipping locked decoy output")
				continue
			}
			decoys = append(decoys, out)
		}
	}
	return decoys, nil
}

// ringMedian returns monerod's median of the ring member indices (the decoys
// plus the real output), matching epee::misc_utils::median: the middle element
// for odd counts, the average of the two middle elements for even counts.
func ringMedian(decoys []DecoyOutput, realGlobalIndex uint64) uint64 {
	idx := make([]uint64, 0, len(decoys)+1)
	for _, d := range decoys {
		idx = append(idx, d.GlobalIndex)
	}
	idx = append(idx, realGlobalIndex)
	sort.Slice(idx, func(i, j int) bool { return idx[i] < idx[j] })
	n := len(idx)
	if n%2 == 1 {
		return idx[n/2]
	}
	return (idx[n/2-1] + idx[n/2]) / 2
}

// selectDecoyIndices picks random output indices, avoiding any already-selected
// indices in the given set (the real output plus prior picks). Chosen indices
// are added to the set so repeated calls never return duplicates.
//
// Indices follow a recency-biased power law: idx = total * (1 - u^3) for
// uniform u. This approximates the gamma distribution real wallets use (most
// ring members recent, long tail into history) and keeps the ring's median
// index above the daemon's relay sanity threshold (median >= 60% of all
// outputs): P(idx > 0.6*total) = 0.74 per draw.
func selectDecoyIndices(totalOutputs uint64, selected map[uint64]bool, count int) []uint64 {
	indices := make([]uint64, 0, count)
	maxAttempts := count * 20

	for len(indices) < count && maxAttempts > 0 {
		maxAttempts--

		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		u := float64(binary.BigEndian.Uint64(randBytes)) / float64(math.MaxUint64)
		idx := uint64(float64(totalOutputs) * (1 - u*u*u))

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

// getRctOutputCount returns the total number of RCT (amount 0) outputs on the
// chain, via a get_output_distribution query over just the last few blocks
// with cumulative counts.
func (c *Client) getRctOutputCount(ctx context.Context) (uint64, error) {
	height, err := c.getBlockCount(ctx)
	if err != nil {
		return 0, err
	}
	// Stay a few blocks behind the tip: the daemon fails the query for heights
	// it has not finalized the distribution for yet.
	from := uint64(0)
	if height > 10 {
		from = height - 10
	}
	result, err := c.jsonRPCRequest(ctx, "get_output_distribution", map[string]interface{}{
		"amounts":     []uint64{0},
		"from_height": from,
		"to_height":   from,
		"cumulative":  true,
		"binary":      false,
	})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Distributions []struct {
			Distribution []uint64 `json:"distribution"`
		} `json:"distributions"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, err
	}
	if len(resp.Distributions) == 0 || len(resp.Distributions[0].Distribution) == 0 {
		return 0, fmt.Errorf("empty output distribution")
	}
	// Cumulative values already include the pre-window base.
	d := resp.Distributions[0].Distribution
	return d[len(d)-1], nil
}

// fetchOutputs retrieves output data (public key and commitment) for given global indices.
func (c *Client) fetchOutputs(ctx context.Context, indices []uint64) ([]DecoyOutput, error) {
	getOuts := make([]map[string]uint64, len(indices))
	for i, idx := range indices {
		getOuts[i] = map[string]uint64{"amount": 0, "index": idx}
	}

	result, err := c.httpRequest(ctx, "/get_outs", map[string]interface{}{
		"outputs":  getOuts,
		"get_txid": false,
	})
	if err != nil {
		return nil, fmt.Errorf("get_outs failed: %w", err)
	}

	var outsResp struct {
		Outs []struct {
			Key      string `json:"key"`
			Mask     string `json:"mask"`
			Txid     string `json:"txid"`
			Height   uint64 `json:"height"`
			Unlocked bool   `json:"unlocked"`
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
			Unlocked:    out.Unlocked,
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
