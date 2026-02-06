package types

const (
	CODE_SUCCESSFUL = "successful"
)

type ApiError struct {
	Message string `json:"error"`
	Code    string `json:"code"`
}

func (e *ApiError) HasError() bool {
	return (e.Message != "" || e.Code != "") && e.Code != CODE_SUCCESSFUL
}

func (e *ApiError) Error() string {
	if e.Code != "" {
		return e.Message + " (code: " + e.Code + ")"
	}
	return e.Message
}

type ApiResponse[T any] struct {
	Data T `json:"data"`
	ApiError
}

type BalanceData struct {
	Balance string `json:"balance"`
}

type BalanceResponse = ApiResponse[BalanceData]

type TokenData struct {
	TokenIdentifier string `json:"tokenIdentifier"`
	Balance         string `json:"balance"`
	Properties      string `json:"properties"`
}

type TokenBalanceData struct {
	TokenData TokenData `json:"tokenData"`
}

type TokenBalanceResponse = ApiResponse[TokenBalanceData]

type TokenProperties struct {
	Type              string `json:"type"`
	Identifier        string `json:"identifier"`
	Name              string `json:"name"`
	Ticker            string `json:"ticker"`
	Owner             string `json:"owner"`
	Decimals          int    `json:"decimals"`
	IsPaused          bool   `json:"isPaused"`
	CanUpgrade        bool   `json:"canUpgrade"`
	CanMint           bool   `json:"canMint"`
	CanBurn           bool   `json:"canBurn"`
	CanChangeOwner    bool   `json:"canChangeOwner"`
	CanPause          bool   `json:"canPause"`
	CanFreeze         bool   `json:"canFreeze"`
	CanWipe           bool   `json:"canWipe"`
	Supply            string `json:"supply"`
	CirculatingSupply string `json:"circulatingSupply"`
}

type Transaction struct {
	TxHash        string             `json:"txHash"`
	GasLimit      uint64             `json:"gasLimit"`
	GasPrice      uint64             `json:"gasPrice"`
	GasUsed       uint64             `json:"gasUsed"`
	MiniBlockHash string             `json:"miniBlockHash"`
	Nonce         uint64             `json:"nonce"`
	Receiver      string             `json:"receiver"`
	ReceiverShard uint32             `json:"receiverShard"`
	Round         uint64             `json:"round"`
	Sender        string             `json:"sender"`
	SenderShard   uint32             `json:"senderShard"`
	Signature     string             `json:"signature"`
	Status        string             `json:"status"`
	Value         string             `json:"value"`
	Fee           string             `json:"fee"`
	Timestamp     int64              `json:"timestamp"`
	Data          string             `json:"data"`
	Function      string             `json:"function"`
	Action        *TransactionAction `json:"action,omitempty"`
	Operations    []Operation        `json:"operations,omitempty"`
	Logs          *TransactionLogs   `json:"logs,omitempty"`
}

type TransactionAction struct {
	Category    string                 `json:"category"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`
}

type Operation struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	Type       string `json:"type"`
	Sender     string `json:"sender"`
	Receiver   string `json:"receiver"`
	Value      string `json:"value"`
	Identifier string `json:"identifier,omitempty"`
	Decimals   int    `json:"decimals,omitempty"`
}

type TransactionLogs struct {
	ID      string             `json:"id"`
	Address string             `json:"address"`
	Events  []TransactionEvent `json:"events"`
}

type TransactionEvent struct {
	Address    string   `json:"address"`
	Identifier string   `json:"identifier"`
	Topics     []string `json:"topics"`
	Data       string   `json:"data"`
	Order      int      `json:"order"`
}

type SubmitTxRequest struct {
	Nonce     uint64 `json:"nonce"`
	Value     string `json:"value"`
	Receiver  string `json:"receiver"`
	Sender    string `json:"sender"`
	GasPrice  uint64 `json:"gasPrice"`
	GasLimit  uint64 `json:"gasLimit"`
	Data      []byte `json:"data,omitempty"`
	ChainID   string `json:"chainID"`
	Version   uint32 `json:"version"`
	Options   uint32 `json:"options,omitempty"`
	Guardian  string `json:"guardian,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type SubmitTxData struct {
	TxHash string `json:"txHash"`
}

type SubmitTxResponse = ApiResponse[SubmitTxData]

type Account struct {
	Address string `json:"address"`
	Nonce   uint64 `json:"nonce"`
	Balance string `json:"balance"`
}

type NetworkConfig struct {
	ChainID        string `json:"erd_chain_id"`
	MinGasPrice    uint64 `json:"erd_min_gas_price"`
	MinGasLimit    uint64 `json:"erd_min_gas_limit"`
	GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
}

type NetworkConfigData struct {
	Config NetworkConfig `json:"config"`
}

type NetworkConfigResponse struct {
	Data NetworkConfigData `json:"data"`
}

type NetworkStatusData struct {
	CurrentRound int64 `json:"erd_current_round"`
}

type NetworkStatusResponse struct {
	Data struct {
		Status NetworkStatusData `json:"status"`
	} `json:"data"`
}

type Block struct {
	Hash             string   `json:"hash"`
	Epoch            uint64   `json:"epoch"`
	Nonce            uint64   `json:"nonce"`
	PrevHash         string   `json:"prevHash"`
	Round            uint64   `json:"round"`
	Shard            uint32   `json:"shard"`
	Timestamp        int64    `json:"timestamp"`
	TxCount          uint64   `json:"txCount"`
	MiniBlocksHashes []string `json:"miniBlocksHashes"`
	StateRootHash    string   `json:"stateRootHash"`
	Size             uint64   `json:"size"`
	SizeTxs          uint64   `json:"sizeTxs"`
	GasConsumed      uint64   `json:"gasConsumed"`
	GasRefunded      uint64   `json:"gasRefunded"`
	GasPenalized     uint64   `json:"gasPenalized"`
	MaxGasLimit      uint64   `json:"maxGasLimit"`
}

type MiniBlockTransaction struct {
	TxHash string `json:"txHash"`
}

// Gateway API types for hyperblock endpoint
type HyperblockResponse struct {
	Data struct {
		Hyperblock Hyperblock `json:"hyperblock"`
	} `json:"data"`
	Error string `json:"error"`
	Code  string `json:"code"`
}

type Hyperblock struct {
	Hash        string       `json:"hash"`
	Nonce       uint64       `json:"nonce"`
	Round       uint64       `json:"round"`
	Epoch       uint64       `json:"epoch"`
	NumTxs      uint64       `json:"numTxs"`
	Timestamp   int64        `json:"timestamp"`
	ShardBlocks []ShardBlock `json:"shardBlocks"`
}

type ShardBlock struct {
	Hash   string `json:"hash"`
	Nonce  uint64 `json:"nonce"`
	Round  uint64 `json:"round"`
	Shard  uint32 `json:"shard"`
}

// Gateway API types for shard block endpoint
type GatewayBlockResponse struct {
	Data struct {
		Block GatewayBlock `json:"block"`
	} `json:"data"`
	Error string `json:"error"`
	Code  string `json:"code"`
}

type GatewayBlock struct {
	Nonce      uint64         `json:"nonce"`
	Round      uint64         `json:"round"`
	Epoch      uint64         `json:"epoch"`
	Shard      uint32         `json:"shard"`
	NumTxs     uint64         `json:"numTxs"`
	Hash       string         `json:"hash"`
	Timestamp  int64          `json:"timestamp"`
	MiniBlocks []GatewayMiniBlock `json:"miniBlocks"`
}

type GatewayMiniBlock struct {
	Hash         string                 `json:"hash"`
	Transactions []GatewayTransaction `json:"transactions"`
}

type GatewayTransaction struct {
	Hash string `json:"hash"`
}
