package eos

import (
	"encoding/json"
	"time"
)

type TransactionResp struct {
	ID      Checksum256 `json:"id"`
	Receipt struct {
		Status            TransactionStatus `json:"status"`
		CPUUsageMicrosec  int               `json:"cpu_usage_us"`
		NetUsageWords     int               `json:"net_usage_words"`
		PackedTransaction TransactionWithID `json:"trx"`
	} `json:"receipt"`
	Transaction           ProcessedTransaction `json:"trx"`
	BlockTime             BlockTimestamp       `json:"block_time"`
	BlockNum              uint32               `json:"block_num"`
	LastIrreversibleBlock uint32               `json:"last_irreversible_block"`
	Traces                []ActionTrace        `json:"traces"`
}

var _ DownloadedTransaction = &TransactionResp{}

func (t *TransactionResp) GetBlockNum() uint64 {
	return uint64(t.BlockNum)
}

func (t *TransactionResp) GetBlockId() string {
	// ??
	return ""
}

func (t *TransactionResp) GetBlockTime() time.Time {
	return t.BlockTime.Time
}

func (t *TransactionResp) GetTxId() string {
	return t.ID.String()
}

func (t *TransactionResp) GetActions() []TxAction {
	actions := make([]TxAction, len(t.Traces))
	for i, trace := range t.Traces {
		actions[i] = &trace
	}
	return actions
}

func (t *TransactionResp) Validate() error {
	return nil
}

type ProcessedTransaction struct {
	Transaction SignedTransaction `json:"trx"`
}

type ActionTraceReceipt struct {
	Receiver        AccountName                    `json:"receiver"`
	ActionDigest    Checksum256                    `json:"act_digest"`
	GlobalSequence  Uint64                         `json:"global_sequence"`
	ReceiveSequence Uint64                         `json:"recv_sequence"`
	AuthSequence    []TransactionTraceAuthSequence `json:"auth_sequence"` // [["account", sequence], ["account", sequence]]
	CodeSequence    Varuint32                      `json:"code_sequence"`
	ABISequence     Varuint32                      `json:"abi_sequence"`
}

type ActionTrace struct {
	ActionOrdinal                          Varuint32           `json:"action_ordinal"`
	CreatorActionOrdinal                   Varuint32           `json:"creator_action_ordinal"`
	ClosestUnnotifiedAncestorActionOrdinal Varuint32           `json:"closest_unnotified_ancestor_action_ordinal"`
	Receipt                                *ActionTraceReceipt `json:"receipt,omitempty" eos:"optional"`
	Receiver                               AccountName         `json:"receiver"`
	Action                                 *Action             `json:"act"`
	ContextFree                            bool                `json:"context_free"`
	Elapsed                                Int64               `json:"elapsed"`
	Console                                SafeString          `json:"console"`
	TransactionID                          Checksum256         `json:"trx_id"`
	BlockNum                               uint32              `json:"block_num"`
	BlockTime                              BlockTimestamp      `json:"block_time"`
	ProducerBlockID                        Checksum256         `json:"producer_block_id" eos:"optional"`
	AccountRAMDeltas                       []*AccountRAMDelta  `json:"account_ram_deltas"`
	Except                                 *Except             `json:"except,omitempty" eos:"optional"`
	ErrorCode                              *Uint64             `json:"error_code,omitempty" eos:"optional"`

	// Not present in EOSIO >= 1.8.x
	InlineTraces []ActionTrace `json:"inline_traces,omitempty" eos:"-"`
}

// Action
type Action struct {
	Account       AccountName       `json:"account"`
	Name          ActionName        `json:"name"`
	Authorization []PermissionLevel `json:"authorization,omitempty"`
	ActionData
}

var _ TxAction = &ActionTrace{}

func (t *ActionTrace) Ok() bool {
	return t.ErrorCode == nil && t.Receipt != nil && t.Action != nil
}

func (t *ActionTrace) GetId() string {
	return t.Receipt.ActionDigest.String()
}

func (t *ActionTrace) GetName() string {
	return string(t.Action.Name)
}

func (t *ActionTrace) GetData() json.RawMessage {
	jsonData, _ := json.Marshal(t.Action.Data)
	return jsonData
}

func (t *ActionTrace) GetAccount() string {
	return string(t.Action.Account)
}
