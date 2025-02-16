package api

type V2GetShardsResponse struct {
	Ok     bool          `json:"ok"`
	Result V2BlockShards `json:"result"`
}

type V2BlockShards struct {
	Type   string                 `json:"@type"`
	Shards []*V2BlockIdExtNoExtra `json:"shards"`
	Extra  ExtraString            `json:"@extra"`
}
