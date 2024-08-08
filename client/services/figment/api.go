package figment

import "time"

type CreateValidatorRequest struct {
	Network           string `json:"network"`
	ValidatorsCount   int    `json:"validators_count"`
	WithdrawalAddress string `json:"withdrawal_address"`
}

type Status string

const (
	Provisioned           Status = "provisioned"
	FundingRequested      Status = "funding_requested"
	DepositedNotFinalized Status = "deposited_not_finalized"
	Deposited             Status = "deposited"
	PendingInitialized    Status = "pending_initialized"
	PendingQueued         Status = "pending_queued"
	ActiveExiting         Status = "active_exiting"
	ActiveOngoing         Status = "active_ongoing"
	ActiveSlashed         Status = "active_slashed"
	ExitedSlashed         Status = "exited_slashed"
	ExitedUnslashed       Status = "exited_unslashed"
	WithdrawalDone        Status = "withdrawal_done"
	WithdrawalPossible    Status = "withdrawal_possible"
)

// Define the struct to match the JSON structure
type ValidatorData struct {
	Network               string          `json:"network"`
	Pubkey                string          `json:"pubkey"`
	Status                Status          `json:"status"`
	WithdrawalAddress     string          `json:"withdrawal_address"`
	WithdrawalCredentials string          `json:"withdrawal_credentials"`
	NetFeePayoutAddress   string          `json:"net_fee_payout_address"`
	FeeRecipientAddress   string          `json:"fee_recipient_address"`
	Region                string          `json:"region"`
	DepositData           DepositData     `json:"deposit_data"`
	StatusHistory         StatusHistory   `json:"status_history"`
	StatusEstimates       StatusEstimates `json:"status_estimates"`
	OnDemandExit          OnDemandExit    `json:"on_demand_exit"`
	ExitMessage           ExitMessage     `json:"exit_message"`
	StakingRequest        StakingRequest  `json:"staking_request"`
}

type DepositData struct {
	DepositDataRoot    string `json:"deposit_data_root"`
	DepositMessageRoot string `json:"deposit_message_root"`
	ForkVersion        string `json:"fork_version"`
	Signature          string `json:"signature"`
	FigmentSignature   string `json:"figment_signature"`
	DepositCLIVersion  string `json:"deposit_cli_version"`
	Amount             int64  `json:"amount"`
}

type StatusHistory struct {
	Events []Event `json:"events"`
}

type Event struct {
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}

type StatusEstimates struct {
	EstimatedActiveAt     *time.Time `json:"estimated_active_at"`
	EstimatedExitAt       *time.Time `json:"estimated_exit_at"`
	EstimatedWithdrawalAt *time.Time `json:"estimated_withdrawal_at"`
}

type OnDemandExit struct {
	RequestedAt *time.Time `json:"requested_at"`
	ApprovedAt  *time.Time `json:"approved_at"`
	SubmittedAt *time.Time `json:"submitted_at"`
	RequestID   *string    `json:"request_id"`
}

type ExitMessage struct {
	EncryptedValue *string `json:"encrypted_value"`
	ForkVersion    *string `json:"fork_version"`
}

type StakingRequest struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type Meta struct {
	StakingRequest     StakingRequestMeta     `json:"staking_request"`
	StakingTransaction StakingTransactionMeta `json:"staking_transaction"`
}

type StakingRequestMeta struct {
	ID                string `json:"id"`
	CreatedAt         string `json:"created_at"`
	AmountWei         string `json:"amount_wei"`
	WithdrawalAddress string `json:"withdrawal_address"`
	Network           string `json:"network"`
	Region            string `json:"region"`
}

type StakingTransactionMeta struct {
	From                          string `json:"from"`
	To                            string `json:"to"`
	AmountWei                     string `json:"amount_wei"`
	ContractCallData              string `json:"contract_call_data"`
	UnsignedTransactionHashed     string `json:"unsigned_transaction_hashed"`
	UnsignedTransactionSerialized string `json:"unsigned_transaction_serialized"`
	MaxGasWei                     string `json:"max_gas_wei"`
}

type CreateValidatorResponse struct {
	Data []ValidatorData `json:"data"`
	Meta Meta            `json:"meta"`
}

type GetValidatorResponse struct {
	Data ValidatorData `json:"data"`
}

type GetValidatorsResponse struct {
	Data []ValidatorData `json:"data"`
}

type ExitValidatorsRequest struct {
	Network           string `json:"network"`
	ValidatorsCount   int    `json:"validators_count"`
	WithdrawalAddress string `json:"withdrawal_address"`
}

type ExitValidatorsPubkeyRequest struct {
	Network string   `json:"network"`
	Pubkeys []string `json:"pubkeys"`
}
