package localtypes

import (
	"encoding/json"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	cometbytes "github.com/cometbft/cometbft/libs/bytes"
	comettypes "github.com/cometbft/cometbft/types"
)

//
// These are copied from cometbft's ResultTx, adds:
// - evm_tx_info, to permit strict unmarshal from SEI chain
//

type ResultTx struct {
	Hash     cometbytes.HexBytes `json:"hash"`
	Height   int64               `json:"height"`
	Index    uint32              `json:"index"`
	TxResult ExecTxResult        `json:"tx_result"`
	Tx       comettypes.Tx       `json:"tx"`
	Proof    comettypes.TxProof  `json:"proof,omitempty"`
}

type ResultTxSearch struct {
	Txs        []*ResultTx `json:"txs"`
	TotalCount int         `json:"total_count"`
}

type ExecTxResult struct {
	Code      uint32            `json:"code,omitempty"`
	Data      []byte            `json:"data,omitempty"`
	Log       string            `json:"log,omitempty"`
	Info      string            `json:"info,omitempty"`
	GasWanted int64             `json:"gas_wanted,omitempty"`
	GasUsed   int64             `json:"gas_used,omitempty"`
	Events    []abcitypes.Event `json:"events,omitempty"`
	Codespace string            `json:"codespace,omitempty"`
	// Added for SEI
	EvmTxInfo json.RawMessage `json:"evm_tx_info,omitempty"`
}
