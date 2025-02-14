package api

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
	Type      string `json:"@type"`
	Workchain int    `json:"workchain"`
	Shard     string `json:"shard"`
	Seqno     int64  `json:"seqno"`
	RootHash  string `json:"root_hash"`
	FileHash  string `json:"file_hash"`
	Extra     string `json:"@extra"`
}
