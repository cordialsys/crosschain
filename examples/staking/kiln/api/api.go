package api

import "time"

type GetAccountsResponse struct {
	Data []Account `json:"data"`
}

type GetAccountResponse struct {
	Data Account `json:"data"`
}

type Account struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Root structure to hold the data array
type CreateValidatorKeysResponse1 struct {
	Data []ValidatorKey `json:"data"`
}
type CreateValidatorKeysResponse2 struct {
	Data BatchValidatorKeys `json:"data"`
}

// DepositData structure to hold individual deposit information
type ValidatorKey struct {
	Format                string `json:"format"`
	PubKey                string `json:"pubkey"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	Amount                int64  `json:"amount"`
	Signature             string `json:"signature"`
	DepositMessageRoot    string `json:"deposit_message_root"`
	DepositDataRoot       string `json:"deposit_data_root"`
	ForkVersion           string `json:"fork_version"`
	NetworkName           string `json:"network_name"`
	DepositCLIVersion     string `json:"deposit_cli_version"`
}

// BatchDepositData structure to hold batch deposit information
type BatchValidatorKeys struct {
	Format                string   `json:"format"`
	PubKeys               []string `json:"pubkeys"`
	WithdrawalCredentials []string `json:"withdrawal_credentials"`
	Signatures            []string `json:"signatures"`
	DepositDataRoots      []string `json:"deposit_data_roots"`
}

type DepositFormat string

var BatchDeposit DepositFormat = "batch_deposit"

// Deposit structure to hold deposit information
type CreateValidatorKeysRequest struct {
	Format              DepositFormat `json:"format"`
	AccountID           string        `json:"account_id"`
	WithdrawalAddress   string        `json:"withdrawal_address"`
	FeeRecipientAddress string        `json:"fee_recipient_address"`
	Number              int           `json:"number"`
}

// Root structure to hold the data object
type GenerateTransactionResponse struct {
	Data TransactionData `json:"data"`
}

// TransactionData structure to hold transaction information
type TransactionData struct {
	UnsignedTxHash          string `json:"unsigned_tx_hash"`
	UnsignedTxSerialized    string `json:"unsigned_tx_serialized"`
	To                      string `json:"to"`
	ContractCallData        string `json:"contract_call_data"`
	AmountWei               string `json:"amount_wei"`
	Nonce                   int    `json:"nonce"`
	GasLimit                int    `json:"gas_limit"`
	MaxPriorityFeePerGasWei string `json:"max_priority_fee_per_gas_wei"`
	MaxFeePerGasWei         string `json:"max_fee_per_gas_wei"`
	ChainID                 int    `json:"chain_id"`
}

type GenerateTransactionRequest struct {
	AccountID string `json:"account_id"`
	Wallet    string `json:"wallet"`
	AmountWei string `json:"amount_wei"`
}
