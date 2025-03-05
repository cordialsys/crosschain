package taostats

type AccountAddress struct {
	SS58 string `json:"ss58"`
	Hex  string `json:"hex"`
}

type AccountData struct {
	Address                 AccountAddress `json:"address"`
	Network                 string         `json:"network"`
	BlockNumber             int64          `json:"block_number"`
	Timestamp               string         `json:"timestamp"`
	Rank                    int            `json:"rank"`
	BalanceFree             string         `json:"balance_free"`
	BalanceStaked           string         `json:"balance_staked"`
	BalanceStakedAlphaAsTao string         `json:"balance_staked_alpha_as_tao"`
	BalanceStakedRoot       string         `json:"balance_staked_root"`
	BalanceTotal            string         `json:"balance_total"`
	CreatedOnDate           string         `json:"created_on_date"`
	CreatedOnNetwork        string         `json:"created_on_network"`
	ColdkeySwap             *string        `json:"coldkey_swap"`
}

type AccountResponse struct {
	Pagination Pagination    `json:"pagination"`
	Data       []AccountData `json:"data"`
}
