package rest

import (
	xc "github.com/cordialsys/crosschain"
)

func (fee *FeeEstimateResponse) GetMostNormalFeeEstimate() xc.AmountBlockchain {
	// try normal
	if len(fee.NormalBuckets) > 0 {
		feeRate := fee.NormalBuckets[0].Feerate
		if feeRate != nil {
			return xc.NewAmountBlockchainFromUint64(uint64(*feeRate))
		}
	}
	// use priority
	if fee.PriorityBucket.Feerate != nil {
		feeRate := fee.PriorityBucket.Feerate
		if feeRate != nil {
			return xc.NewAmountBlockchainFromUint64(uint64(*feeRate))
		}
	}

	// use default
	return xc.NewAmountBlockchainFromUint64(1)

}
