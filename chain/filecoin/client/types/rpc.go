package types

import (
	"encoding/json"
)

const (
	MethodChainGetBlock                     = "Filecoin.ChainGetBlock"
	MethodChainGetMessage                   = "Filecoin.ChainGetMessage"
	MethodChainGetParentMessages            = "Filecoin.ChainGetParentMessages"
	MethodChainGetTipSet                    = "Filecoin.ChainGetTipSet"
	MethodChainGetTipSetAfterHeight         = "Filecoin.ChainGetTipSetAfterHeight"
	MethodChainHead                         = "Filecoin.ChainHead"
	MethodEthGetMessageCidByTransactionHash = "Filecoin.EthGetMessageCidByTransactionHash"
	MethodGasEstimateMessageGas             = "Filecoin.GasEstimateMessageGas"
	MethodMpoolGetNonce                     = "Filecoin.MpoolGetNonce"
	MethodMpoolPush                         = "Filecoin.MpoolPush"
	MethodStateSearchMsg                    = "Filecoin.StateSearchMsg"
	MethodWalletBallance                    = "Filecoin.WalletBalance"
)

func NewEmptyParams(method string) Params[[]byte] {
	return Params[[]byte]{
		JsonRpc: "2.0",
		Method:  method,
		Id:      1,
	}
}

// Filecoin uses position based parameters. Because of that, we have to
// create a custom MarshalJSON methods for some parameters.
type Params[T any] struct {
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Id      int    `json:"id"`
	Params  T      `json:"params"`
}

func NewParams[T any](method string, params T) Params[T] {
	return Params[T]{
		JsonRpc: "2.0",
		Method:  method,
		Id:      1,
		Params:  params,
	}
}

type Error struct {
	Code    int         `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	jsonE, _ := json.Marshal(e)
	return string(jsonE)
}

type Response[T any] struct {
	Error  Error `json:"error,omitempty"`
	Result *T    `json:"result,omitempty"`
}

func NewResponse[T any]() *Response[T] {
	return &Response[T]{}
}

func (r Response[T]) IsError() bool {
	return r.Error.Code != 0
}

type WalletBalance struct {
	Address string
}

func (wb *WalletBalance) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{wb.Address})
}

type Cid struct {
	Value string `json:"/,omitempty"`
}

func NewCid(cid string) Cid {
	return Cid{Value: cid}
}

type ChainGetMessage struct {
	// All filecoin CID params are stored in `{ "/": "cid" }` object
	Cid Cid
}

func (cgm *ChainGetMessage) MarshalJSON() ([]byte, error) {
	wrapped := []Cid{cgm.Cid}
	return json.Marshal(wrapped)
}

type Message struct {
	Version    int    `json:"Version"`
	To         string `json:"To"`
	From       string `json:"From"`
	Nonce      uint64 `json:"Nonce"`
	Value      string `json:"Value"`
	GasLimit   uint64 `json:"GasLimit"`
	GasFeeCap  string `json:"GasFeeCap"`
	GasPremium string `json:"GasPremium"`
	Method     int    `json:"Method"`
	Params     []byte `json:"Params"`
}

type ChainGetMessageResponse Message

func NewChainGetMessageResponse() *Response[ChainGetMessageResponse] {
	return NewResponse[ChainGetMessageResponse]()
}

// TipsetKey is a collection of block CIDs
type TipsetKey []Cid

type ChainHeadResponse struct {
	TipsetKey TipsetKey `json:"Cids"`
	Height    uint64    `json:"Height"`
}

func NewChainHeadResponse() *Response[ChainHeadResponse] {
	return NewResponse[ChainHeadResponse]()
}

type StateSearchMsg struct {
	TipSetKey     TipsetKey
	Message       Cid
	Limit         int
	AllowReplaced bool
}

func (ssm *StateSearchMsg) MarshalJSON() ([]byte, error) {
	arr := []interface{}{ssm.TipSetKey, ssm.Message, ssm.Limit, ssm.AllowReplaced}
	return json.Marshal(arr)
}

type Receipt struct {
	ExitCode   int    `json:"ExitCode"`
	Return     string `json:"Return"`
	GasUsed    uint64 `json:"GasUsed"`
	EventsRoot Cid    `json:"EventsRoot"`
}

type StateSearchMsgResponse struct {
	Message Cid       `json:"Message"`
	Receipt Receipt   `json:"Receipt"`
	TipSet  TipsetKey `json:"TipSet"`
	Height  int       `json:"Height"`
}

func NewStateSearchMsgResponse() *Response[StateSearchMsgResponse] {
	return NewResponse[StateSearchMsgResponse]()
}

type ChainGetBlock struct {
	// All filecoin CID params are stored in `{ "/": "cid" }` object
	Cid Cid
}

func (cgb *ChainGetBlock) MarshalJSON() ([]byte, error) {
	wrapped := []Cid{cgb.Cid}
	return json.Marshal(wrapped)
}

type ChainGetBlockResponse struct {
	Timestamp     int64  `json:"Timestamp"`
	Height        uint64 `json:"Height"`
	ParentBaseFee string `json:"ParentBaseFee"`
	Parents       []Cid  `json:"Parents"`
}

func NewChainGetBlockResponse() *Response[ChainGetBlockResponse] {
	return NewResponse[ChainGetBlockResponse]()
}

type MaxFee struct {
	MaxFee string `json:"MaxFee"`
}

type GasEstimateMessageGas struct {
	Message   Message
	MaxFee    MaxFee
	TipsetKey TipsetKey
}

func (gemg *GasEstimateMessageGas) MarshalJSON() ([]byte, error) {
	array := []interface{}{gemg.Message, gemg.MaxFee, gemg.TipsetKey}
	return json.Marshal(array)
}

type GasEstimateMessageGasResponse Message

func NewGasEstimateMessageGasResponse() *Response[GasEstimateMessageGasResponse] {
	return NewResponse[GasEstimateMessageGasResponse]()
}

type MpoolGetNonce struct {
	Address string
}

func (mgn *MpoolGetNonce) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{mgn.Address})
}

type MpoolGetNonceResponse int

func NewMpoolGetNonceResponse() *Response[MpoolGetNonceResponse] {
	return NewResponse[MpoolGetNonceResponse]()
}

type Signature struct {
	Type byte   `json:"Type"`
	Data []byte `json:"Data"`
}

type MpoolPush struct {
	Message   Message   `json:"Message"`
	Signature Signature `json:"Signature"`
}

func (mp *MpoolPush) MarshalJSON() ([]byte, error) {
	data := struct {
		Message   Message   `json:"Message"`
		Signature Signature `json:"Signature"`
	}{
		mp.Message, mp.Signature,
	}

	return json.Marshal([]interface{}{
		data,
	})
}

type MpoolPushResponse Cid

func NewMpoolPushResponse() *Response[MpoolPushResponse] {
	return NewResponse[MpoolPushResponse]()
}

type ChainGetTipSetAfterHeight struct {
	Height uint64
}

func (tp *ChainGetTipSetAfterHeight) MarshalJSON() ([]byte, error) {
	params := []interface{}{tp.Height, []int64{}}
	return json.Marshal(params)
}

type ChainGetTipSetAfterHeightResponse ChainHeadResponse

func NewChainGetTipSetAfterHeightResponse() *Response[ChainGetTipSetAfterHeightResponse] {
	return NewResponse[ChainGetTipSetAfterHeightResponse]()
}

type ChainGetParentMessages struct {
	Cid Cid
}

func (gpm *ChainGetParentMessages) MarshalJSON() ([]byte, error) {
	wrapped := []Cid{gpm.Cid}
	return json.Marshal(wrapped)
}

type CidMessage struct {
	Cid     Cid     `json:"Cid"`
	Message Message `json:"Message"`
}

type ChainGetParentMessagesResponse []CidMessage

func NewChainGetParentMessagesResponse() *Response[ChainGetParentMessagesResponse] {
	return NewResponse[ChainGetParentMessagesResponse]()
}

type EthGetMessageCidByTransactionHash string

func (hash *EthGetMessageCidByTransactionHash) MarshalJSON() ([]byte, error) {
	wrapped := []string{string(*hash)}
	return json.Marshal(wrapped)
}

type EvmTxHashToCidResponse Cid

func NewEvmTxHashToCidResponse() *Response[EvmTxHashToCidResponse] {
	return NewResponse[EvmTxHashToCidResponse]()
}
