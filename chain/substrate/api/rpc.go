package api

import "github.com/centrifuge/go-substrate-rpc-client/v4/types"

type InclusionFee struct {
	AdjustedWeightFee string `json:"adjustedWeightFee"`
	BaseFee           string `json:"baseFee"`
	LenFee            string `json:"lenFee"`
}

type FeeEstimateResponse struct {
	InclusionFee InclusionFee `json:"inclusionFee"`
}

// AccountInfo contains a subset of what a parachain may return in order to maximize decoding iteroperability.
// To see other fields, see types.AccountInfo
type AccountInfoMinimal struct {
	Nonce       types.U32
	Consumers   types.U32
	Providers   types.U32
	Sufficients types.U32
	Data        struct {
		Free types.U128
		// skip fields after this point as we don't need them
	}
}

// // Redefine the block without having to parse all of the extrinsics.
// // This is useful if all we care about is computing the extrinsic hash.
// type SignedRawBlock struct {
// 	Block         RawBlock            `json:"block"`
// 	Justification types.Justification `json:"justification"`
// }

// // Block without parsing the extrinsics (leaving them as hex-encoded-scale)
// type RawBlock struct {
// 	Header     types.Header
// 	Extrinsics []string
// }

// // May need to add to this list
// type EventRecords struct {
// 	types.EventRecords
// }
