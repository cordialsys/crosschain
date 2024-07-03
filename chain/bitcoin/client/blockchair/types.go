package blockchair

import (
	"encoding/json"

	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
)

type blockchairStatsData struct {
	Blocks       uint64  `json:"blocks"`
	SuggestedFee float64 `json:"suggested_transaction_fee_per_byte_sat"`
}

type blockchairStats struct {
	Data    blockchairStatsData `json:"data"`
	Context BlockchairContext   `json:"context"`
}

type blockchairAddressFull struct {
	ScriptHex string `json:"script_hex"`
	Balance   uint64 `json:"balance"`
}

type blockchairTransactionFull struct {
	Hash    string `json:"hash"`
	Time    string `json:"time"`
	Fee     uint64 `json:"fee"`
	BlockId int64  `json:"block_id"`
}

var _ tx_input.UtxoI = blockchairUTXO{}

func (u blockchairUTXO) GetValue() uint64 {
	return u.Value
}
func (u blockchairUTXO) GetBlock() uint64 {
	if u.Block < 0 {
		return 0
	}
	return uint64(u.Block)
}
func (u blockchairUTXO) GetTxHash() string {
	return u.TxHash
}
func (u blockchairUTXO) GetIndex() uint32 {
	return u.Index
}

type blockchairUTXO struct {
	// BlockId uint64  `json:"block_id"`
	TxHash  string `json:"transaction_hash"`
	Index   uint32 `json:"index"`
	Value   uint64 `json:"value"`
	Address string `json:"address,omitempty"`
	// This will be >0 if the UTXO is confirmed
	Block int64 `json:"block_id"`
}

type blockchairOutput struct {
	blockchairUTXO
	Recipient string `json:"recipient"`
	ScriptHex string `json:"script_hex"`
}

type blockchairInput struct {
	blockchairOutput
}

type BlockchairContext struct {
	Code  int32  `json:"code"` // 200 = ok
	Error string `json:"error,omitempty"`
	State int64  `json:"state"` // to count confirmations
}

type blockchairTransactionData struct {
	Transaction blockchairTransactionFull `json:"transaction"`
	Inputs      []blockchairInput         `json:"inputs"`
	Outputs     []blockchairOutput        `json:"outputs"`
}

type blockchairAddressData struct {
	// Transactions []blockchairTransaction `json:"transactions"`
	Address blockchairAddressFull `json:"address"`
	Utxo    []blockchairUTXO      `json:"utxo"`
}

type blockchairData struct {
	Data    map[string]json.RawMessage `json:"data"`
	Context BlockchairContext          `json:"context"`
}
type blockchairNotFoundData struct {
	Data    []string          `json:"data"`
	Context BlockchairContext `json:"context"`
}
