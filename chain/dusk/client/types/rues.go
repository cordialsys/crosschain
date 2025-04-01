package types

import (
	"encoding/json"
	"fmt"
)

const (
	POST                = "POST"
	TARGET_CONTRACT     = "contracts"
	TARGET_ACCOUNT      = "account"
	TARGET_GRAPHQL      = "graphql"
	TARGET_TRANSACTIONS = "transactions"
	TOPIC_CHAIN_ID      = "chain_id"
	TOPIC_STATUS        = "status"
	TOPIC_QUERY         = "query"
	TOPIC_PROPAGATE     = "propagate"
	TOPIC_VERIFY        = "preverify"
	TRANSFER_CONTRACT   = "0100000000000000000000000000000000000000000000000000000000000000"
)

// RuesRequest describes a request to the Dusk network.
// Params representation varies:
// - For GraphQL requests, it is a query string of standard GraphQL request
// - Other endpoints can use custom formats, please refer to documentation or rusk-wallet source code
//
// https://docs.dusk.network/developer/integrations/rues
type RuesRequest struct {
	Method string
	Target string
	Entity string
	Topic  string
	Params []byte
}

func NewGraphQlRequest(params []byte, ruskSessionId string) RuesRequest {
	return RuesRequest{
		Method: POST,
		Target: TARGET_GRAPHQL,
		Entity: "",
		Topic:  TOPIC_QUERY,
		Params: params,
	}
}

func (rr *RuesRequest) IsGraphQL() bool {
	return rr.Target == TARGET_GRAPHQL
}

// URL schema: {rpc-url}/on/{target}:{entity}/{topic}
// `:entity` part is optional and should be omitted if `entity` is empty
func (rr *RuesRequest) GetUrl(base string) string {
	if rr.Entity != "" {
		return fmt.Sprintf("%s/%s:%s/%s", base, rr.Target, rr.Entity, rr.Topic)
	} else {
		return fmt.Sprintf("%s/%s/%s", base, rr.Target, rr.Topic)
	}
}

func (rr *RuesRequest) GetParams() []byte {
	return rr.Params
}

type GetAccountStatusResult struct {
	// Account balance in blockchain amount
	Balance uint64 `json:"balance"`
	// Last transaction Nonce
	Nonce uint64 `json:"nonce"`
}

type GetBlockParams struct {
	Height uint64
}

func (rr *GetBlockParams) ToBytesParams() []byte {
	return []byte(fmt.Sprintf(`query {block(height:%d) { header { hash, height, timestamp } , transactions {id}}}`, rr.Height))
}

type TransactionId struct {
	Id string `json:"id"`
}

type BlockHeader struct {
	Height    uint64 `json:"height"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
}

type Block struct {
	Header       BlockHeader     `json:"header"`
	Transactions []TransactionId `json:"transactions"`
}

type GetLastBlockParams struct{}

func (rr *GetLastBlockParams) ToBytesParams() []byte {
	return []byte("query {lastBlockPair { json }}")
}

type GetBlockResult struct {
	Block Block `json:"block"`
}

type BlockPair struct {
	// Array of `[height, hash]`
	// Example: "last_finalized_block": [664,"transaction_hash"]
	LastFinalizedBlock []interface{} `json:"last_finalized_block"`
}

type LastBlockPair struct {
	Json BlockPair `json:"json"`
}

type LastBlockPairResult struct {
	LastBlockPair LastBlockPair `json:"lastBlockPair"`
}

type GetTransactionParams struct {
	Id string
}

func (rr *GetTransactionParams) ToBytesParams() []byte {
	return []byte(fmt.Sprintf("query { tx(hash:\"%s\") { err, tx { json }, gasSpent, blockHash, blockHeight, blockTimestamp, id }}", rr.Id))
}

type Fee struct {
	GasPrice string `json:"gas_price"`
}

type Transaction struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Value    uint64 `json:"value"`
	Fee      Fee    `json:"fee"`
}

type TransactionJson struct {
	Json string `json:"json"`
}

func (t *TransactionJson) GetTransaction() (Transaction, error) {
	var tx Transaction
	err := json.Unmarshal([]byte(t.Json), &tx)
	return tx, err
}

type SpentTransaction struct {
	Tx             TransactionJson `json:"tx"`
	Err            string          `json:"err,omitempty"`
	GasSpent       uint64          `json:"gasSpent"`
	BlockHash      string          `json:"blockHash"`
	BlockHeight    uint64          `json:"blockHeight"`
	BlockTimestamp int64           `json:"blockTimestamp"`
	ID             string          `json:"id"`
	Raw            string          `json:"raw"`
}

type GetTransactionResult struct {
	SpentTransaction *SpentTransaction `json:"tx,omitempty"`
}

type GetChainIdParams struct {
}
