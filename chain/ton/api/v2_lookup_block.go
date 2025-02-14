package api

type V2BlockResponse struct {
	Ok     bool         `json:"ok"`
	Result V2BlockIdExt `json:"result"`
}
