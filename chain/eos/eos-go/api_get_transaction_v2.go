package eos

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type TransactionV2Resp struct {
	QueryTimeMs          float64               `json:"query_time_ms"`
	Executed             bool                  `json:"executed"`
	TrxID                string                `json:"trx_id"`
	Lib                  uint64                `json:"lib"`
	CachedLib            bool                  `json:"cached_lib"`
	Actions              []TransactionV2Action `json:"actions"`
	LastIndexedBlock     uint64                `json:"last_indexed_block"`
	LastIndexedBlockTime string                `json:"last_indexed_block_time"`
}

var _ DownloadedTransaction = &TransactionV2Resp{}

func (t *TransactionV2Resp) Validate() error {
	if len(t.Actions) == 0 {
		return fmt.Errorf("no actions")
	}

	return nil
}

func (t *TransactionV2Resp) GetBlockNum() uint64 {
	return t.Actions[0].BlockNum
}

func (t *TransactionV2Resp) GetBlockId() string {
	return t.Actions[0].BlockID
}

func (t *TransactionV2Resp) GetBlockTime() time.Time {
	blockTime, err := time.Parse("2006-01-02T15:04:05", t.Actions[0].Timestamp)
	if err != nil {
		blockTime, err = time.Parse(time.RFC3339, t.Actions[0].Timestamp)
		if err != nil {
			return time.Time{}
		}
	}
	return blockTime
}

func (t *TransactionV2Resp) GetTxId() string {
	return t.TrxID
}

func (t *TransactionV2Resp) GetActions() []TxAction {
	actions := make([]TxAction, len(t.Actions))
	for i, action := range t.Actions {
		actions[i] = &action
	}
	return actions
}

type TransactionV2Action struct {
	ActionOrdinal        int                    `json:"action_ordinal"`
	CreatorActionOrdinal int                    `json:"creator_action_ordinal"`
	Act                  TransactionV2ActInner  `json:"act"`
	Timestamp            string                 `json:"@timestamp"`
	BlockNum             uint64                 `json:"block_num"`
	BlockID              string                 `json:"block_id"`
	Producer             string                 `json:"producer"`
	TrxID                string                 `json:"trx_id"`
	GlobalSequence       uint64                 `json:"global_sequence"`
	CPUUsageUs           int                    `json:"cpu_usage_us"`
	NetUsageWords        int                    `json:"net_usage_words"`
	Signatures           []string               `json:"signatures"`
	InlineCount          int                    `json:"inline_count"`
	InlineFiltered       bool                   `json:"inline_filtered"`
	Receipts             []TransactionV2Receipt `json:"receipts"`
	CodeSequence         int                    `json:"code_sequence"`
	ABISequence          int                    `json:"abi_sequence"`
	ActDigest            string                 `json:"act_digest"`
	Time                 string                 `json:"timestamp"`
}

type TransactionV2ActInner struct {
	Account       string              `json:"account"`
	Name          string              `json:"name"`
	Authorization []TransactionV2Auth `json:"authorization"`
	Data          json.RawMessage     `json:"data"`
}

var _ TxAction = &TransactionV2Action{}

func (t *TransactionV2Action) GetId() string {
	return t.ActDigest
}
func (t *TransactionV2Action) GetName() string {
	return t.Act.Name
}

func (t *TransactionV2Action) GetData() json.RawMessage {
	return t.Act.Data
}

func (t *TransactionV2Action) GetAccount() string {
	return t.Act.Account
}

func (t *TransactionV2Action) Ok() bool {
	return true
}

type TransactionV2Auth struct {
	Actor      string `json:"actor"`
	Permission string `json:"permission"`
}

type TransactionV2Receipt struct {
	Receiver       string                 `json:"receiver"`
	GlobalSequence string                 `json:"global_sequence"`
	RecvSequence   string                 `json:"recv_sequence"`
	AuthSequence   []TransactionV2AuthSeq `json:"auth_sequence"`
}

type TransactionV2AuthSeq struct {
	Account  string `json:"account"`
	Sequence string `json:"sequence"`
}

// This is a hyperion endpoint
func (api *API) GetTransactionV2(ctx context.Context, id string) (out *TransactionV2Resp, err error) {
	err = api.callv2(ctx, "GET", "history", "get_transaction", M{"id": id}, &out)
	return
}
