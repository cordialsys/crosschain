package client

import "testing"

// TestRingMedianMatchesMonerod checks ringMedian against monerod's
// epee::misc_utils::median (avg of the two middle elements for even counts),
// using the exact ring of the real testnet tx that monerod rejected with
// "Sanity check failed" (transactions/8). The daemon requires the median to be
// >= 60% of total rct outputs; this ring's median was ~57%.
func TestRingMedianMatchesMonerod(t *testing.T) {
	// tx 8 ring: 15 decoys + the real output (7348946), 16 members.
	decoys := []DecoyOutput{
		{GlobalIndex: 1551407}, {GlobalIndex: 2118977}, {GlobalIndex: 2801681},
		{GlobalIndex: 2832333}, {GlobalIndex: 2984435}, {GlobalIndex: 3236512},
		{GlobalIndex: 3371243}, {GlobalIndex: 4072805}, {GlobalIndex: 4322818},
		{GlobalIndex: 5814657}, {GlobalIndex: 5867839}, {GlobalIndex: 6745048},
		{GlobalIndex: 7140480}, {GlobalIndex: 7227340}, {GlobalIndex: 7346529},
	}
	const real = 7348946
	// sorted 8th+9th (1-indexed) = 4072805, 4322818
	want := uint64((4072805 + 4322818) / 2)
	if got := ringMedian(decoys, real); got != want {
		t.Fatalf("ringMedian = %d, want %d", got, want)
	}

	// This is what the daemon rejected: median below 60% of ~7.349M outputs.
	const totalOutputs = 7348950
	if ringMedian(decoys, real) >= totalOutputs*6/10 {
		t.Fatalf("expected this ring's median to fail the 60%% sanity threshold")
	}
}

// TestRingMedianRecentPasses confirms a recency-heavy ring clears the threshold.
func TestRingMedianRecentPasses(t *testing.T) {
	const totalOutputs = 7348950
	target := uint64(totalOutputs) * 63 / 100
	// 15 decoys all in the recent zone => median must clear the 60% floor.
	decoys := make([]DecoyOutput, 0, 15)
	for i := 0; i < 15; i++ {
		decoys = append(decoys, DecoyOutput{GlobalIndex: target + uint64(i)*1000})
	}
	if ringMedian(decoys, 100) < totalOutputs*6/10 {
		t.Fatalf("recent ring should pass the sanity threshold")
	}
}

// TestOldRealKeepsLowDecoys is the P2 regression test. When spending an OLD
// output, the ring must still contain low-index decoys so the real output is
// not a conspicuous low outlier — the previous median-fixer stripped exactly
// those decoys. It also confirms the recency-biased draw clears the daemon's
// 60% median floor the vast majority of the time (so the whole-ring re-roll in
// FetchDecoys is rarely needed) without any per-member editing.
func TestOldRealKeepsLowDecoys(t *testing.T) {
	const (
		total    = uint64(8_000_000)
		count    = 15
		oldReal  = uint64(80_000) // 1% of the chain — a deliberately old output
		trials   = 2000
		floor    = total * 6 / 10
		lowZone  = total * 4 / 10 // "old" region a stripper would have emptied
		recentZn = total / 2
	)

	passFloor := 0
	keptLowDecoy := 0
	recentDecoys := 0
	totalDecoys := 0

	for i := 0; i < trials; i++ {
		selected := map[uint64]bool{oldReal: true}
		idx := selectDecoyIndices(total, selected, count)
		if len(idx) != count {
			t.Fatalf("expected %d decoys, got %d", count, len(idx))
		}
		decoys := make([]DecoyOutput, len(idx))
		hasLow := false
		for j, gi := range idx {
			decoys[j] = DecoyOutput{GlobalIndex: gi}
			if gi == oldReal {
				t.Fatalf("decoy collided with the real output index")
			}
			totalDecoys++
			if gi < lowZone {
				hasLow = true
			}
			if gi > recentZn {
				recentDecoys++
			}
		}
		if hasLow {
			keptLowDecoy++
		}
		if ringMedian(decoys, oldReal) >= floor {
			passFloor++
		}
	}

	// The old real output gets low-index company in the large majority of rings,
	// instead of being stripped down to a lone outlier.
	if keptLowDecoy < trials*70/100 {
		t.Errorf("expected most rings to keep a low decoy near the old real; got %d/%d", keptLowDecoy, trials)
	}
	// Recency-biased draw clears the daemon's median floor without any fixing.
	if passFloor < trials*75/100 {
		t.Errorf("expected most rings to clear the 60%% median floor; got %d/%d", passFloor, trials)
	}
	// Sanity: the distribution is still recency-heavy overall.
	if recentDecoys < totalDecoys*55/100 {
		t.Errorf("expected the draw to remain recency-biased; %d/%d decoys were recent", recentDecoys, totalDecoys)
	}
}
