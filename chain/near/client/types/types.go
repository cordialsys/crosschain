package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	xc "github.com/cordialsys/crosschain"
)

const (
	FinalityFinal                  = "final"
	FunctionCallMethodTransfer     = "ft_transfer"
	FunctionCallMethodTransferCall = "ft_transfer_call"
	KeyAccountId                   = "account_id"
	KeyBlockId                     = "block_id"
	KeyFinality                    = "finality"
	KeyRequestType                 = "request_type"
	MethodNameFtBalanceOf          = "ft_balance_of"
	MethodNameFtMetadata           = "ft_metadata"
	MethodNameStorageBalanceOf     = "storage_balance_of"
	MethodNameStorageBalanceBounds = "storage_balance_bounds"
	RequestTypeCallFunction        = "call_function"
	RequestTypeViewAccount         = "view_account"
	RequestTypeViewAccessKeyList   = "view_access_key_list"
	WaitUntilExecuted              = "EXECUTED"
)

type Params interface {
	ToParams() (any, error)
}

type Response[T any] struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  T      `json:"result"`
	Err     *Error `json:"error"`
}

type Error struct {
	Name  string
	Cause any
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %+v", e.Name, e.Cause)
}

func toMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return result, nil
}

type callFunctionParams struct {
	RequestType string `json:"request_type,omitempty"`
	Finality    string `json:"finality,omitempty"`
	AccountId   string `json:"account_id,omitempty"`
	MethodName  string `json:"method_name,omitempty"`
	ArgsBase64  string `json:"args_base64,omitempty"`
}

type ViewAccountParams struct {
	callFunctionParams
}

func NewViewAccountParams(acc string) ViewAccountParams {
	return ViewAccountParams{
		callFunctionParams{
			AccountId:   acc,
			Finality:    FinalityFinal,
			RequestType: RequestTypeViewAccount,
		},
	}
}

func (v ViewAccountParams) ToParams() (any, error) {
	return toMap(v)
}

type FtBalanceOfParams struct {
	callFunctionParams
}

func NewFtBalanceOfParams(acc string, contract string) (FtBalanceOfParams, error) {
	type args struct {
		AccountId string `json:"account_id"`
	}
	a, err := json.Marshal(args{AccountId: acc})
	if err != nil {
		return FtBalanceOfParams{}, err
	}

	return FtBalanceOfParams{
		callFunctionParams{
			RequestType: RequestTypeCallFunction,
			Finality:    FinalityFinal,
			AccountId:   contract,
			MethodName:  MethodNameFtBalanceOf,
			ArgsBase64:  base64.StdEncoding.EncodeToString(a),
		},
	}, nil
}

func (f FtBalanceOfParams) ToParams() (any, error) {
	return toMap(f)
}

type TokenMetadataParams struct {
	callFunctionParams
}

func NewTokenMetadataParams(contract string) (TokenMetadataParams, error) {
	return TokenMetadataParams{
		callFunctionParams{
			RequestType: RequestTypeCallFunction,
			Finality:    FinalityFinal,
			AccountId:   contract,
			MethodName:  MethodNameFtMetadata,
			ArgsBase64:  base64.StdEncoding.EncodeToString([]byte("{}")),
		},
	}, nil
}

func (t TokenMetadataParams) ToParams() (any, error) {
	return toMap(t)
}

type TokenMetadataResult struct {
	Result []byte `json:"result"`
}

func (t TokenMetadataResult) GetTokenMetadata() (TokenMetadata, error) {
	var tm TokenMetadata
	err := json.Unmarshal(t.Result, &tm)
	return tm, err
}

type TokenMetadata struct {
	Spec     string `json:"spec"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type FtBalanceOf struct {
	Result []byte   `json:"result"`
	Logs   []string `json:"logs"`
}

type ViewAccessKeyListParams struct {
	callFunctionParams
}

func NewViewAccessKeyListParams(acc string) ViewAccessKeyListParams {
	return ViewAccessKeyListParams{
		callFunctionParams{
			RequestType: RequestTypeViewAccessKeyList,
			Finality:    FinalityFinal,
			AccountId:   acc,
		},
	}
}

func (v ViewAccessKeyListParams) ToParams() (any, error) {
	return toMap(v)
}

type Key struct {
	PublicKey string    `json:"public_key"`
	AccessKey AccessKey `json:"access_key"`
}

type AccessKeyList struct {
	Keys      []Key  `json:"keys"`
	BlockHash string `json:"block_hash"`
}

type Account struct {
	Amount string `json:"amount"`
}

// Block can be fetched by hash, id, or finality
// Unfortunaltely both hash and id are reffered to as `block_id`, so we have to handle
// the conversion to int in the `ToParams` method
type BlockParams struct {
	Finality *string `json:"finality,omitempty"`
	BlockId  *string `json:"block_id,omitempty"`
}

func (b BlockParams) ToParams() (any, error) {
	params, err := toMap(b)
	if err != nil {
		return nil, err
	}

	block_id, ok := params[KeyBlockId]
	// determine if by hash or height
	if ok {
		strBlockId := block_id.(string)
		blockHeight, err := strconv.Atoi(strBlockId)
		if err == nil {
			params[KeyBlockId] = blockHeight
		}
	}
	return params, nil
}

func NewBlockParamsById(id string) BlockParams {
	return BlockParams{
		Finality: nil,
		BlockId:  &id,
	}
}

func NewLatestBlockParams() BlockParams {
	final := "final"
	return BlockParams{
		Finality: &final,
		BlockId:  nil,
	}
}

type ChunkParams struct {
	ChunkId string `json:"chunk_id"`
}

func (c ChunkParams) ToParams() (any, error) {
	return toMap(c)
}

type ChunkStatus struct {
	Header       ChunkHeader
	Transactions []Transaction
}

type ChunkHeader struct {
	ChunkHash string `json:"chunk_hash"`
}

type Block struct {
	Header BlockHeader   `json:"header"`
	Chunks []ChunkHeader `json:"chunks"`
}

type BlockHeader struct {
	Hash      string `json:"hash"`
	Height    uint64 `json:"height"`
	Timestamp uint64 `json:"timestamp"`
}

type TxStatusParams struct {
	TxHash          string `json:"tx_hash"`
	SenderAccountId string `json:"sender_account_id"`
}

func (p TxStatusParams) ToParams() (any, error) {
	return toMap(p)
}

type TxStatus struct {
	FinalExecutionStatus string            `json:"final_execution_status"`
	Status               TransactionStatus `json:"status"`
	Transaction          Transaction       `json:"transaction"`
	TransactionOutcome   Outcome           `json:"transaction_outcome"`
	Receipts             []Receipt         `json:"receipts"`
	ReceiptsOutcome      []ReceiptOutcome  `json:"receipts_outcome"`
}

type ActionError struct {
	Index int `json:"index"`
	// NEAR transaction errors are verbose and have inconsistent structure.
	// Error kinds can contain strings, nested objects, or empty objects (used as flags).
	// Using map[string]any avoids modeling dozens of error-specific structs while
	// allowing flexible error handling by checking map keys.
	Kind map[string]any `json:"kind"`
}

type Failure struct {
	ActionError ActionError `json:"ActionError"`
}

type TransactionStatus struct {
	SuccessValue     string   `json:"SuccessValue,omitempty"`
	SuccessReceiptId string   `json:"SuccessReceiptId,omitempty"`
	Failure          *Failure `json:"Failure,omitempty"`
}

func (s TransactionStatus) GetError() (*string, error) {
	if s.Failure == nil {
		return nil, nil
	}

	for kind, e := range s.Failure.ActionError.Kind {
		txError := new(string)
		if e != nil {
			jsonErr, err := json.Marshal(e)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal action error: %w", err)
			}
			*txError = fmt.Sprintf("%s: %s", kind, string(jsonErr))
		} else {
			*txError = kind
		}

		return txError, nil
	}

	return nil, nil
}

type Transaction struct {
	Hash        string   `json:"hash"`
	Nonce       uint64   `json:"nonce"`
	PriorityFee uint64   `json:"priority_fee"`
	PublicKey   string   `json:"public_key"`
	ReceiverID  string   `json:"receiver_id"`
	Signature   string   `json:"signature"`
	SignerID    string   `json:"signer_id"`
	Actions     []Action `json:"actions"`
}

type Action struct {
	CreateAccount *struct{}           `json:"CreateAccount,omitempty"`
	Transfer      *TransferAction     `json:"Transfer,omitempty"`
	AddKey        *AddKeyAction       `json:"AddKey,omitempty"`
	FunctionCall  *FunctionCallAction `json:"FunctionCall,omitempty"`
}

type TransferAction struct {
	Deposit string `json:"deposit"`
}

type AddKeyAction struct {
	PublicKey string    `json:"public_key"`
	AccessKey AccessKey `json:"access_key"`
}

type FunctionCallArgs struct {
	ReceiverID string `json:"receiver_id"`
	Amount     string `json:"amount"`
	Memo       string `json:"memo,omitempty"`
	Msg        string `json:"msg,omitempty"`
}

type FunctionCallAction struct {
	MethodName string `json:"method_name"`
	Args       string `json:"args"` // base64 encoded
	Gas        uint64 `json:"gas"`
	Deposit    string `json:"deposit"`
}

func (fc FunctionCallAction) IsTransfer() bool {
	return fc.MethodName == FunctionCallMethodTransfer || fc.MethodName == FunctionCallMethodTransferCall
}

type AccessKey struct {
	Nonce      uint64 `json:"nonce"`
	Permission string `json:"permission"`
}

type Outcome struct {
	BlockHash string        `json:"block_hash"`
	ID        string        `json:"id"`
	Outcome   OutcomeDetail `json:"outcome"`
	Proof     []MerkleProof `json:"proof"`
}

type OutcomeDetail struct {
	ExecutorID  string            `json:"executor_id"`
	GasBurnt    uint64            `json:"gas_burnt"`
	Logs        []string          `json:"logs"`
	ReceiptIDs  []string          `json:"receipt_ids"`
	Status      TransactionStatus `json:"status"`
	TokensBurnt string            `json:"tokens_burnt"`
}

type Receipt struct {
	PredecessorID string        `json:"predecessor_id"` // ← Different from SignerID
	ReceiverID    string        `json:"receiver_id"`
	ReceiptID     string        `json:"receipt_id"` // ← Has its own ID
	Priority      uint64        `json:"priority"`
	Receipt       ReceiptDetail `json:"receipt"` // ← Nested structure!
}

type ReceiptDetail struct {
	Action *ReceiptAction `json:"Action,omitempty"`
	Data   *ReceiptData   `json:"Data,omitempty"`
}

type ReceiptAction struct {
	Actions         []Action `json:"actions"` // ← Same Action type
	GasPrice        string   `json:"gas_price"`
	SignerID        string   `json:"signer_id"` // ← Original signer
	SignerPublicKey string   `json:"signer_public_key"`
	IsPromiseYield  bool     `json:"is_promise_yield"`
}

type ReceiptData struct {
	DataID string `json:"data_id"`
	Data   []byte `json:"data"`
}

type ReceiptOutcome struct {
	BlockHash string        `json:"block_hash"`
	ID        string        `json:"id"`
	Outcome   OutcomeDetail `json:"outcome"`
	Proof     []MerkleProof `json:"proof"`
}

type MerkleProof struct {
	Direction string `json:"direction"`
	Hash      string `json:"hash"`
}

// Helper to unmarshal Actions which can be strings or objects
func (a *Action) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "CreateAccount" {
			a.CreateAccount = &struct{}{}
		}
		return nil
	}

	type Alias Action
	aux := &struct{ *Alias }{Alias: (*Alias)(a)}
	return json.Unmarshal(data, &aux)
}

type SendTxParams struct {
	SignedTxBase64 string `json:"signed_tx_base64"`
	WaitUntil      string `json:"wait_until"`
}

func NewSendTxParams(signedTxBase64 string) SendTxParams {
	return SendTxParams{
		SignedTxBase64: signedTxBase64,
		WaitUntil:      WaitUntilExecuted,
	}
}

func (s SendTxParams) ToParams() (any, error) {
	return toMap(s)
}

type StorageBalanceOfParams struct {
	callFunctionParams
}

func NewStorageBalanceOfParams(contract string, account_id string) (StorageBalanceOfParams, error) {
	args, err := json.Marshal(map[string]any{
		KeyAccountId: account_id,
	})
	if err != nil {
		return StorageBalanceOfParams{}, fmt.Errorf("failed to marshal storage balance of params: %w", err)
	}

	return StorageBalanceOfParams{
		callFunctionParams: callFunctionParams{
			RequestType: RequestTypeCallFunction,
			Finality:    FinalityFinal,
			AccountId:   contract,
			MethodName:  MethodNameFtBalanceOf,
			ArgsBase64:  base64.StdEncoding.EncodeToString(args),
		},
	}, nil
}

func (s StorageBalanceOfParams) ToParams() (any, error) {
	return toMap(s)
}

type StorageBalanceResult struct {
	Result []byte `json:"result"`
}

func (s StorageBalanceResult) GetStorageBalance() (xc.AmountBlockchain, error) {
	if s.Result == nil {
		return xc.NewAmountBlockchainFromUint64(0), nil
	}

	// Some contracts return StorageBalance struct, and some *string
	var sb StorageBalance
	err := json.Unmarshal(s.Result, &sb)
	if err != nil {
		var balance string
		err := json.Unmarshal(s.Result, &balance)
		return xc.NewAmountBlockchainFromStr(balance), err
	}

	return xc.NewAmountBlockchainFromStr(sb.Available), nil
}

type StorageBalance struct {
	Total     string `json:"total"`
	Available string `json:"available"`
}

type StorageBalanceBoundsParams struct {
	callFunctionParams
}

func NewStorageBalanceBoundsParams(contract string, acc string) (StorageBalanceBoundsParams, error) {
	type args struct {
		AccountId string `json:"account_id"`
	}
	a, err := json.Marshal(args{AccountId: acc})
	if err != nil {
		return StorageBalanceBoundsParams{}, err
	}

	return StorageBalanceBoundsParams{
		callFunctionParams: callFunctionParams{
			RequestType: RequestTypeCallFunction,
			Finality:    FinalityFinal,
			AccountId:   contract,
			MethodName:  MethodNameStorageBalanceBounds,
			ArgsBase64:  base64.StdEncoding.EncodeToString(a),
		},
	}, nil
}

func (s StorageBalanceBoundsParams) ToParams() (any, error) {
	return toMap(s)
}

type StorageBalanceBoundsResult struct {
	Result []byte
}

func (s StorageBalanceBoundsResult) GetStorageBalanceBounds() (StorageBalanceBounds, error) {
	var sbb StorageBalanceBounds
	err := json.Unmarshal(s.Result, &sbb)
	return sbb, err
}

type StorageBalanceBounds struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

type ProtocolConfigParams struct {
}

func (p ProtocolConfigParams) ToParams() (any, error) {
	return map[string]any{
		KeyFinality: FinalityFinal,
	}, nil
}

type GasCost struct {
	Execution  uint64 `json:"execution"`
	SendNotSir uint64 `json:"send_not_sir"`
	SendSir    uint64 `json:"send_sir"`
}

type ActionCreationConfig struct {
	TransferCost     GasCost `json:"transfer_cost"`
	FunctionCallCost GasCost `json:"function_call_cost"`
}

type TransactionCosts struct {
	ActionCreationConfig        ActionCreationConfig `json:"action_creation_config"`
	ActionReceiptCreationConfig GasCost              `json:"action_receipt_creation_config"` // fixed typo
}

type RuntimeConfig struct {
	TransactionCosts TransactionCosts `json:"transaction_costs"`
}

type ProtocolConfig struct {
	RuntimeConfig RuntimeConfig `json:"runtime_config"`
}

type GasPriceParams struct{}

func (g GasPriceParams) ToParams() (any, error) {
	return []*uint8{nil}, nil
}

type GasPrice struct {
	GasPrice string `json:"gas_price"`
}
