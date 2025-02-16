package api

import (
	"fmt"
	"strconv"
	"strings"
)

type V2MasterchainInfoResponse struct {
	Ok     bool        `json:"ok"`
	Result V2BlockInfo `json:"result"`
}

type V2BlockInfo struct {
	Type          string       `json:"@type"`
	Last          V2BlockIdExt `json:"last"`
	StateRootHash string       `json:"state_root_hash"`
	Init          V2BlockIdExt `json:"init"`
}

type V2BlockIdExt struct {
	Type      string      `json:"@type"`
	Workchain int         `json:"workchain"`
	Shard     string      `json:"shard"`
	Seqno     int64       `json:"seqno"`
	RootHash  string      `json:"root_hash"`
	FileHash  string      `json:"file_hash"`
	Extra     ExtraString `json:"@extra"`
}
type V2BlockIdExtNoExtra struct {
	Type      string `json:"@type"`
	Workchain int    `json:"workchain"`
	Shard     string `json:"shard"`
	Seqno     int64  `json:"seqno"`
	RootHash  string `json:"root_hash"`
	FileHash  string `json:"file_hash"`
}

type ExtraString string

// They have blocktime thrown in this adhoc string, e.g. "1739542296.249538:1:0.3620847591001577"
func (extra ExtraString) Timestamp() (float64, error) {
	extraParts := strings.Split(string(extra), ":")
	blockTimeString := extraParts[0]
	blockTimeFloat, err := strconv.ParseFloat(blockTimeString, 64)
	if err != nil {
		return blockTimeFloat, fmt.Errorf("invalid 'extra' field on block: %s: %v", extra, err)
	}
	return blockTimeFloat, nil
}
