package api

type GetAccountResponse struct {
	Balance             string `json:"balance"`
	Code                string `json:"code"`
	Data                string `json:"data"`
	LastTransactionLt   string `json:"last_transaction_lt"`
	LastTransactionHash string `json:"last_transaction_hash"`
	FrozenHash          string `json:"frozen_hash"`
	Status              string `json:"status"`
}

type Detail struct {
	Loc  []interface{} `json:"loc"`
	Msg  string        `json:"msg"`
	Type string        `json:"type"`
}

type ErrorResponse struct {
	// API doc specifies "details" as being in the response but in practice
	// it seems just a single error string is returned
	Error  string   `json:"error"`
	Detail []Detail `json:"detail"`
}

type JettonWallet struct {
	Address           string `json:"address"`
	Balance           string `json:"balance"`
	Owner             string `json:"owner"`
	Jetton            string `json:"jetton"`
	LastTransactionLt string `json:"last_transaction_lt"`
	CodeHash          string `json:"code_hash"`
	DataHash          string `json:"data_hash"`
}

type JettonWalletsResponse struct {
	JettonWallets []JettonWallet `json:"jetton_wallets"`
}
