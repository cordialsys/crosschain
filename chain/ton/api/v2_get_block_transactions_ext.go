package api

type V2GetBlockTransactionsResponse struct {
	Ok     bool                `json:"ok"`
	Result V2BlockTransactions `json:"result"`
}

type V2BlockTransactions struct {
	Type         string           `json:"@type"`
	Id           V2BlockIdExt     `json:"id"`
	ReqCount     int              `json:"req_count"`
	Incomplete   bool             `json:"incomplete"`
	Transactions []*V2Transaction `json:"transactions"`
	Extra        string           `json:"@extra"`
}

type V2Transaction struct {
	Type          string                  `json:"@type"`
	Address       V2Account               `json:"address"`
	Utime         int64                   `json:"utime"`
	Data          string                  `json:"data"`
	TransactionId V2InternalTransactionId `json:"transaction_id"`
	Fee           string                  `json:"fee"`
	StorageFee    string                  `json:"storage_fee"`
	OtherFee      string                  `json:"other_fee"`
	InMsg         V2Message               `json:"in_msg"`
	OutMsgs       []*V2Message            `json:"out_msgs"`
	Account       string                  `json:"account"`
}

type V2InternalTransactionId struct {
	Type string `json:"@type"`
	Lt   string `json:"lt"`
	Hash string `json:"hash"`
}
type V2Account struct {
	Type           string `json:"@type"`
	AccountAddress string `json:"account_address"`
}

type V2Message struct {
	Type            string    `json:"@type"`
	Hash            string    `json:"hash"`
	Source          V2Account `json:"source"`
	Destination     V2Account `json:"destination"`
	Value           string    `json:"value"`
	ExtraCurrencies []string  `json:"extra_currencies"`
	FwdFee          string    `json:"fwd_fee"`
	IhrFee          string    `json:"ihr_fee"`
	CreatedLt       string    `json:"created_lt"`
	BodyHash        string    `json:"body_hash"`
	// drop as we don't need
	// MsgData      MsgData  `json:"msg_data"`
}
