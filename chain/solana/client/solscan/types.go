package solscan

import "encoding/json"

type TxDetailResponse struct {
	Success bool          `json:"success"`
	Data    *TxDetailData `json:"data"`
}

type TxDetailData struct {
	TxID               string        `json:"trans_id"`
	BlockID            int64         `json:"block_id"`
	BlockTime          int64         `json:"trans_time"`
	Fee                uint64        `json:"fee"`
	Status             int           `json:"status"`
	Signer             []string      `json:"signer"`
	ListSigner         []string      `json:"list_signer"`
	ParsedInstructions []Instruction `json:"parsed_instructions"`
	TxStatus           string        `json:"txStatus"`
}

type Instruction struct {
	InsIndex          int             `json:"ins_index"`
	OuterInsIndex     *int            `json:"outer_ins_index"`
	Type              string          `json:"type"`
	Transfers         []Transfer      `json:"transfers"`
	DataRaw           json.RawMessage `json:"data_raw"`
	InnerInstructions []Instruction   `json:"inner_instructions"`
}

type InstructionData struct {
	Type string           `json:"type"`
	Info *InstructionInfo `json:"info"`
}

type InstructionInfo struct {
	Source               string      `json:"source"`
	Destination          string      `json:"destination"`
	Lamports             interface{} `json:"lamports"`
	IntentTransferSetter string      `json:"intentTransferSetter"`
}

type Transfer struct {
	SourceAddress        string      `json:"source"`
	DestinationAddress   string      `json:"destination"`
	FromAddress          string      `json:"from"`
	ToAddress            string      `json:"to"`
	SourceOwner          string      `json:"source_owner"`
	DestinationOwner     string      `json:"destination_owner"`
	FromOwner            string      `json:"from_owner"`
	ToOwner              string      `json:"to_owner"`
	Amount               interface{} `json:"amount"`
	Value                interface{} `json:"value"`
	TokenAmount          interface{} `json:"token_amount"`
	Lamports             interface{} `json:"lamports"`
	TokenAddress         string      `json:"token_address"`
	TokenContractAddress string      `json:"token_contract_address"`
	Mint                 string      `json:"mint"`

	InsIndex      int  `json:"ins_index"`
	OuterInsIndex *int `json:"outer_ins_index"`
}

type BlockResponse struct {
	Success bool        `json:"success"`
	Data    []BlockData `json:"data"`
}

type BlockData struct {
	BlockHeight int64  `json:"blockHeight"`
	BlockTime   int64  `json:"blockTime"`
	BlockHash   string `json:"blockHash"`
	ParentSlot  int64  `json:"parentSlot"`
	// ... more fields ...
}

type ChainInfoResponse struct {
	Success bool           `json:"success"`
	Data    *ChainInfoData `json:"data"`
}

type ChainInfoData struct {
	BlockHeight  int64       `json:"blockHeight"`
	CurrentEpoch int64       `json:"currentEpoch"`
	AbsoluteSlot int64       `json:"absoluteSlot"`
	Network      NetworkData `json:"networkInfo"`
	// ... more fields ...
}

type NetworkData struct {
	BlockTime        int64 `json:"blockTime"`
	BlockHeight      int64 `json:"blockHeight"`
	TransactionCount int64 `json:"transactionCount"`
	// ... more fields ...
}

func (info *ChainInfoData) GetBlockHeight() int64 {
	if info.Network.BlockHeight > info.BlockHeight {
		return info.Network.BlockHeight
	}
	return info.BlockHeight
}
