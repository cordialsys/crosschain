package api

type InclusionFee struct {
	AdjustedWeightFee string `json:"adjustedWeightFee"`
	BaseFee           string `json:"baseFee"`
	LenFee            string `json:"lenFee"`
}

type FeeEstimateResponse struct {
	InclusionFee InclusionFee `json:"inclusionFee"`
}
