package api

type V2GetBlockShortTransactionsResponse struct {
	Ok     bool                     `json:"ok"`
	Result V2BlockShortTransactions `json:"result"`
}

type V2BlockShortTransactions struct {
	Type         string         `json:"@type"`
	Id           V2BlockIdExt   `json:"id"`
	ReqCount     int            `json:"req_count"`
	Incomplete   bool           `json:"incomplete"`
	Transactions []*V2ShortTxId `json:"transactions"`
	Extra        string         `json:"@extra"`
}

type V2ShortTxId struct {
	Type    string `json:"@type"`
	Mode    int    `json:"mode"`
	Account string `json:"account"`
	Lt      string `json:"lt"`
	Hash    string `json:"hash"`
}
