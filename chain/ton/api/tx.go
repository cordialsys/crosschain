package api

type MessageContent struct {
	Hash    string   `json:"hash"`
	Body    string   `json:"body"`
	Decoded *Decoded `json:"decoded,omitempty"`
}

type Decoded struct {
	Type    string `json:"type"`
	Comment string `json:"comment"`
}

type InMsg struct {
	Hash           string         `json:"hash"`
	Source         *string        `json:"source"`
	Destination    *string        `json:"destination"`
	Value          *string        `json:"value"`
	FwdFee         *string        `json:"fwd_fee"`
	IhrFee         *string        `json:"ihr_fee"`
	CreatedLt      *string        `json:"created_lt"`
	CreatedAt      *string        `json:"created_at"`
	Opcode         string         `json:"opcode"`
	IhrDisabled    *bool          `json:"ihr_disabled"`
	Bounce         *bool          `json:"bounce"`
	Bounced        *bool          `json:"bounced"`
	ImportFee      *string        `json:"import_fee"`
	MessageContent MessageContent `json:"message_content"`
	InitState      *InitState     `json:"init_state"`
}

type InitState struct {
	Hash string `json:"hash"`
	Body string `json:"body"`
}

type AccountState struct {
	Hash          string  `json:"hash"`
	Balance       string  `json:"balance"`
	AccountStatus string  `json:"account_status"`
	FrozenHash    *string `json:"frozen_hash"`
	CodeHash      *string `json:"code_hash"`
	DataHash      *string `json:"data_hash"`
}

type OutMsg struct {
	InMsg
}

type ComputePh struct {
	Mode             int    `json:"mode"`
	Type             string `json:"type"`
	Success          bool   `json:"success"`
	GasFees          string `json:"gas_fees"`
	GasUsed          string `json:"gas_used"`
	VmSteps          int    `json:"vm_steps"`
	ExitCode         int    `json:"exit_code"`
	GasLimit         string `json:"gas_limit"`
	GasCredit        string `json:"gas_credit"`
	MsgStateUsed     bool   `json:"msg_state_used"`
	AccountActivated bool   `json:"account_activated"`
	VmInitStateHash  string `json:"vm_init_state_hash"`
	VmFinalStateHash string `json:"vm_final_state_hash"`
}

type StoragePh struct {
	StatusChange         string `json:"status_change"`
	StorageFeesCollected string `json:"storage_fees_collected"`
}

type CreditPh struct {
	Credit string `json:"credit"`
}

type Action struct {
	Valid           bool       `json:"valid"`
	Success         bool       `json:"success"`
	NoFunds         bool       `json:"no_funds"`
	ResultCode      int        `json:"result_code"`
	TotActions      int        `json:"tot_actions"`
	MsgsCreated     int        `json:"msgs_created"`
	SpecActions     int        `json:"spec_actions"`
	TotMsgSize      TotMsgSize `json:"tot_msg_size"`
	StatusChange    string     `json:"status_change"`
	TotalFwdFees    string     `json:"total_fwd_fees"`
	SkippedActions  int        `json:"skipped_actions"`
	ActionListHash  string     `json:"action_list_hash"`
	TotalActionFees string     `json:"total_action_fees"`
}

type TotMsgSize struct {
	Bits  string `json:"bits"`
	Cells string `json:"cells"`
}

type Description struct {
	Type        string    `json:"type"`
	Action      Action    `json:"action"`
	Aborted     bool      `json:"aborted"`
	CreditPh    CreditPh  `json:"credit_ph"`
	Destroyed   bool      `json:"destroyed"`
	ComputePh   ComputePh `json:"compute_ph"`
	StoragePh   StoragePh `json:"storage_ph"`
	CreditFirst bool      `json:"credit_first"`
}

type Transaction struct {
	Account            string       `json:"account"`
	Hash               string       `json:"hash"`
	Lt                 string       `json:"lt"`
	Now                int64        `json:"now"`
	OrigStatus         string       `json:"orig_status"`
	EndStatus          string       `json:"end_status"`
	TotalFees          string       `json:"total_fees"`
	PrevTransHash      string       `json:"prev_trans_hash"`
	PrevTransLt        string       `json:"prev_trans_lt"`
	Description        Description  `json:"description"`
	BlockRef           BlockRef     `json:"block_ref"`
	InMsg              InMsg        `json:"in_msg"`
	OutMsgs            []OutMsg     `json:"out_msgs"`
	AccountStateBefore AccountState `json:"account_state_before"`
	AccountStateAfter  AccountState `json:"account_state_after"`
	McBlockSeqno       int64        `json:"mc_block_seqno"`
}

type AddressBookEntry struct {
	UserFriendly string `json:"user_friendly"`
}

type AddressBook map[string]AddressBookEntry

type TransactionsData struct {
	Transactions []Transaction `json:"transactions"`
	AddressBook  AddressBook   `json:"address_book"`
}
