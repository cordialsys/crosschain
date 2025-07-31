package icp

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
	icperrors "github.com/cordialsys/crosschain/chain/internet_computer/client/types/errors"
	"github.com/fxamacker/cbor/v2"
)

const (
	MethodAccountBalance                   = "account_balance"
	MethodQueryBlocks                      = "query_blocks"
	MethodTransfer                         = "transfer"
	MethodGetAccountIdentifierTransactions = "get_account_identifier_transactions"
)

var LedgerPrincipal = address.MustDecode("ryjl3-tyaaa-aaaaa-aaaba-cai")

type GetBalanceArgs struct {
	Account []byte `ic:"account"`
}

type Balance struct {
	E8S uint64 `ic:"e8s"`
}

type QueryBlocksArgs struct {
	Start  uint64 `ic:"start" json:"start"`
	Length uint64 `ic:"length" json:"length"`
}

type ArchivedBlocksRange struct {
	Start    uint64       `ic:"start" json:"start"`
	Length   uint64       `ic:"length" json:"length"`
	Callback idl.Function `ic:"callback" json:"callback"`
}

type Timestamp struct {
	TimestampNanos uint64 `ic:"timestamp_nanos" cbor:"0,keyasint"`
}

func NewTimestamp(timestampNanos uint64) Timestamp {
	return Timestamp{
		TimestampNanos: timestampNanos,
	}
}

func (t Timestamp) ToUnixTime() time.Time {
	seconds := int64(t.TimestampNanos / 1_000_000_000)
	nanos := int64(t.TimestampNanos % 1_000_000_000)

	return time.Unix(seconds, nanos)
}

type Tokens struct {
	E8s uint64 `ic:"e8s" cbor:"0,keyasint"`
}

func NewTokens(amount uint64) Tokens {
	return Tokens{
		E8s: amount,
	}
}

type AddressConstraint interface {
	[]byte | string
}

type Mint[T AddressConstraint] struct {
	To     T      `ic:"to" json:"to"`
	Amount Tokens `ic:"amount" json:"amount"`
}

func (m *Mint[T]) MarshalCBOR() ([]byte, error) {
	type MintCBOR struct {
		To     string `cbor:"0,keyasint"`
		Amount Tokens `cbor:"1,keyasint"`
	}

	switch v := any(m.To).(type) {
	case string:
		mintCBOR := MintCBOR{
			To:     v,
			Amount: m.Amount,
		}
		return cbor.Marshal(mintCBOR)
	case []byte:
		mintCBOR := MintCBOR{
			To:     hex.EncodeToString(v),
			Amount: m.Amount,
		}
		return cbor.Marshal(mintCBOR)
	default:
		panic("unreachable")
	}
}

func (m Mint[T]) GetTo() string {
	switch v := any(m.To).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	default:
		panic("unreachable")
	}
}

type Burn[T AddressConstraint] struct {
	From    T      `ic:"from" json:"from"`
	Amount  Tokens `ic:"amount" json:"amount"`
	Spender *T     `ic:"spender,omitempty" json:"spender,omitempty"`
}

func (b Burn[T]) GetFrom() string {
	switch v := any(b.From).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	default:
		panic("unreachable")
	}
}

func (b Burn[T]) GetSpender() string {
	if b.Spender == nil {
		return ""
	}

	switch v := any(b.Spender).(type) {
	case *string:
		return *v
	case *[]byte:
		return hex.EncodeToString(*v)
	default:
		panic("unreachable")
	}
}

func (b *Burn[T]) MarshalCBOR() ([]byte, error) {
	type BurnCBOR struct {
		From    string `cbor:"0,keyasint"`
		Amount  Tokens `cbor:"1,keyasint"`
		Spender string `cbor:"2,keyasint,omitempty"`
	}
	spender := ""
	if b.Spender != nil {
		switch v := any(b.Spender).(type) {
		case *[]byte:
			spender = hex.EncodeToString(*v)
		case *string:
			spender = *v
		}
	}

	switch v := any(b.From).(type) {
	case []byte:
		burnCBOR := BurnCBOR{
			From:    hex.EncodeToString(v),
			Amount:  b.Amount,
			Spender: spender,
		}
		return cbor.Marshal(burnCBOR)
	case string:
		burnCBOR := BurnCBOR{
			From:    v,
			Amount:  b.Amount,
			Spender: spender,
		}

		return cbor.Marshal(burnCBOR)
	default:
		panic("unreachable")
	}

}

type Approve[T AddressConstraint] struct {
	From              T          `ic:"from" json:"from"`
	Spender           T          `ic:"spender" json:"spender"`
	Allowance         Tokens     `ic:"allowance" json:"allowance"`
	AllowanceE8s      idl.Int    `ic:"allowance_e8s" json:"allowance_e8s"`
	ExpectedAllowance *Tokens    `ic:"expected_allowance,omitempty" json:"expected_allowance,omitempty"`
	ExpiresAt         *Timestamp `ic:"expires_at,omitempty" json:"expires_at,omitempty"`
	Fee               Tokens     `ic:"fee" json:"fee"`
}

func (a Approve[T]) GetFrom() string {
	switch v := any(a.From).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	default:
		panic("unreachable")
	}
}

func (a Approve[T]) GetSpender() string {
	switch v := any(a.Spender).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	default:
		panic("unreachable")
	}
}

func (a *Approve[T]) MarshalCBOR() ([]byte, error) {
	type ApproveCBOR struct {
		From              string     `cbor:"0,keyasint"`
		To                string     `cbor:"1,keyasint"`
		Allowance         Tokens     `cbor:"2,keyasint"`
		ExpectedAllowance *Tokens    `cbor:"3,keyasint"`
		ExpiresAt         *Timestamp `cbor:"4,keyasint"`
		Fee               Tokens     `cbor:"5,keyasint"`
	}

	approveCBOR := ApproveCBOR{
		Allowance:         a.Allowance,
		ExpectedAllowance: a.ExpectedAllowance,
		ExpiresAt:         a.ExpiresAt,
		Fee:               a.Fee,
	}

	switch v := any(a.From).(type) {
	case []byte:
		approveCBOR.From = hex.EncodeToString(v)
	case string:
		approveCBOR.From = v
	default:
		panic("unreachable")
	}

	switch v := any(a.Spender).(type) {
	case []byte:
		approveCBOR.To = hex.EncodeToString(v)
	case string:
		approveCBOR.To = v
	default:
		panic("unreachable")
	}

	return cbor.Marshal(approveCBOR)
}

type Transfer[T AddressConstraint] struct {
	From    T      `ic:"from" json:"from"`
	To      T      `ic:"to" json:"to"`
	Amount  Tokens `ic:"amount" json:"amount"`
	Fee     Tokens `ic:"fee" json:"fee"`
	Spender *T     `ic:"spender,omitempty" json:"spender"`
}

func (t Transfer[T]) GetFrom() string {
	switch v := any(t.From).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	default:
		panic("unreachable")
	}
}

func (t Transfer[T]) GetTo() string {
	switch v := any(t.To).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	default:
		panic("unreachable")
	}
}

func (t Transfer[T]) GetSpender() string {
	if t.Spender == nil {
		return ""
	}
	switch v := any(t.Spender).(type) {
	case *string:
		return *v
	case *[]byte:
		return hex.EncodeToString(*v)
	default:
		panic("unreachable")
	}
}

func (t *Transfer[T]) MarshalCBOR() ([]byte, error) {
	type TransferCBOR struct {
		From    string `ic:"from" cbor:"0,keyasint"`
		To      string `ic:"to" cbor:"1,keyasint"`
		Amount  Tokens `ic:"amount" cbor:"2,keyasint"`
		Fee     Tokens `ic:"fee" cbor:"3,keyasint"`
		Spender string `ic:"spender,omitempty" cbor:"4,keyasint,omitempty"`
	}

	transferCbor := TransferCBOR{
		Amount: t.Amount,
		Fee:    t.Fee,
	}

	spender := ""
	if t.Spender != nil {
		switch v := any(t.Spender).(type) {
		case *[]byte:
			spender = hex.EncodeToString(*v)
		case *string:
			spender = *v
		}
	}
	transferCbor.Spender = spender

	switch v := any(t.From).(type) {
	case []byte:
		transferCbor.From = hex.EncodeToString(v)
	case string:
		transferCbor.From = v
	}

	switch v := any(t.To).(type) {
	case []byte:
		transferCbor.To = hex.EncodeToString(v)
	case string:
		transferCbor.To = v
	}

	return cbor.Marshal(transferCbor)
}

type Operation[T AddressConstraint] struct {
	Burn     *Burn[T]     `ic:"Burn,variant" cbor:"0,keyasint,omitempty"`
	Mint     *Mint[T]     `ic:"Mint,variant" cbor:"1,keyasint,omitempty"`
	Transfer *Transfer[T] `ic:"Transfer,variant" cbor:"2,keyasint,omitempty"`
	Approve  *Approve[T]  `ic:"Approve,variant" cbor:"3,keyasint,omitempty"`
}

func (b Operation[T]) From() string {
	if b.Approve != nil {
		return b.Approve.GetFrom()
	} else if b.Burn != nil {
		return b.Burn.GetFrom()
	} else if b.Mint != nil {
		return ""
	} else if b.Transfer != nil {
		return b.Transfer.GetFrom()
	} else {
		return ""
	}
}

func (b Operation[T]) To() string {
	if b.Approve != nil {
		return ""
	} else if b.Burn != nil {
		return ""
	} else if b.Mint != nil {
		return b.Mint.GetTo()
	} else if b.Transfer != nil {
		return b.Transfer.GetTo()
	} else {
		return ""
	}
}

func (b Operation[T]) Amount() uint64 {
	if b.Approve != nil {
		return b.Approve.Allowance.E8s
	} else if b.Burn != nil {
		return b.Burn.Amount.E8s
	} else if b.Mint != nil {
		return b.Mint.Amount.E8s
	} else if b.Transfer != nil {
		return b.Transfer.Amount.E8s
	} else {
		return 0
	}
}

func (b Operation[T]) Fee() uint64 {
	if b.Approve != nil {
		return b.Approve.Fee.E8s
	} else if b.Burn != nil {
		return 0
	} else if b.Mint != nil {
		return 0
	} else if b.Transfer != nil {
		return b.Transfer.Fee.E8s
	} else {
		return 0
	}
}

type TransactionWithId[T AddressConstraint] struct {
	Id          idl.Nat        `ic:"id"`
	Transaction Transaction[T] `ic:"transaction"`
}

type BlockTransaction struct {
	Operation     *Operation[[]byte] `ic:"operation,omitempty"`
	Memo          uint64             `ic:"memo"`
	CreatedAtTime Timestamp          `ic:"created_at_time,omitempty"`
	Icrc1Memo     *[]byte            `ic:"icrc1_memo,omitempty"`
	Timestamp     *Timestamp         `ic:"timestamp,omitempty"`
}

type Transaction[T AddressConstraint] struct {
	Operation     Operation[T] `ic:"operation,omitempty" cbor:"0,keyasint"`
	Memo          uint64       `ic:"memo" cbor:"1,keyasint"`
	CreatedAtTime *Timestamp   `ic:"created_at_time,omitempty" cbor:"2,keyasint"`
	Icrc1Memo     *[]byte      `ic:"icrc1_memo,omitempty" cbor:"3,keyasint,omitempty"`
	Timestamp     *Timestamp   `ic:"timestamp,omitempty" cbor:"-"`
}

func (tx Transaction[T]) Hash() (string, error) {
	cborData, err := cbor.Marshal(tx)
	if err != nil {
		return "", fmt.Errorf("failed to CBOR marshal transaction: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(cborData)
	hash := hasher.Sum(nil)

	return hex.EncodeToString(hash), nil
}

func (tx Transaction[T]) Amount() Tokens {
	return Tokens{
		E8s: tx.Operation.Amount(),
	}
}

func (tx Transaction[T]) Fee() Tokens {
	return Tokens{
		E8s: tx.Operation.Fee(),
	}
}

func (tx Transaction[T]) SourceAddress() string {
	return tx.Operation.From()
}

func (tx Transaction[T]) DestinationAddress() string {
	return tx.Operation.To()
}

type Block struct {
	ParentHash  *[]byte          `ic:"parent_hash,omitempty" json:"parent_hash,omitempty"`
	Transaction BlockTransaction `ic:"transaction" json:"transaction"`
	Timestamp   Timestamp        `ic:"timestamp" json:"timestamp"`
}

func (b Block) Hash() (string, error) {
	if b.ParentHash == nil {
		return "", errors.New("missing parent hash")
	}

	hasher := sha256.New()
	// Hash parent hash
	hasher.Write(*b.ParentHash)

	// Hash timestamp
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, b.Timestamp.TimestampNanos)
	hasher.Write(timestampBytes)

	// Hash transaction (using Candid encoding)
	txBytes, err := candid.Marshal([]interface{}{b.Transaction})
	if err != nil {
		return "", fmt.Errorf("failed to marshal tx: %w", err)
	}
	hasher.Write(txBytes)

	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash), nil
}

func (b Block) TxHash() (string, error) {
	tx := b.Transaction
	var createdAtTime *Timestamp
	// `get_blocks` and `query_blocks` methods are filling empty `created_at_time` values
	// which in turn breaks tx hash
	if tx.CreatedAtTime.TimestampNanos != b.Timestamp.TimestampNanos {
		createdAtTime = &tx.CreatedAtTime
	}

	t := Transaction[[]byte]{
		Operation:     *tx.Operation,
		Memo:          tx.Memo,
		CreatedAtTime: createdAtTime,
		Icrc1Memo:     tx.Icrc1Memo,
		Timestamp:     tx.Timestamp,
	}
	cborData, err := cbor.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to CBOR marshal transaction: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(cborData)
	hash := hasher.Sum(nil)

	return hex.EncodeToString(hash), nil
}

type QueryBlocksResponse struct {
	ChainLength     uint64                `ic:"chain_length" json:"chain_length"`
	Certificate     *[]byte               `ic:"certificate,omitempty" json:"certificate,omitempty"`
	Blocks          []Block               `ic:"blocks" json:"blocks"`
	FirstBlockIndex uint64                `ic:"first_block_index" json:"first_block_index"`
	ArchivedBlocks  []ArchivedBlocksRange `ic:"archived_blocks" json:"archived_blocks"`
}

type BlockRange struct {
	Blocks []Block `ic:"blocks" json:"blocks"`
}

type GetBlocksError struct {
	BadFirstBlockIndex *struct {
		RequestedIndex  uint64 `ic:"requested_index" json:"requested_index"`
		FirstValidIndex uint64 `ic:"first_valid_index" json:"first_valid_index"`
	} `ic:"BadFirstBlockIndex,variant"`
	Other *struct {
		ErrorCode    uint64 `ic:"error_code" json:"error_code"`
		ErrorMessage string `ic:"error_message" json:"error_message"`
	} `ic:"Other,variant"`
}

type GetBlocksResult struct {
	Ok  *BlockRange     `ic:"Ok,variant"`
	Err *GetBlocksError `ic:"Err,variant"`
}

type TransferArgs struct {
	To             []byte     `ic:"to" json:"to"`
	Fee            Tokens     `ic:"fee" json:"fee"`
	Memo           uint64     `ic:"memo" json:"memo"`
	FromSubaccount *[]byte    `ic:"from_subaccount,omitempty" json:"from_subaccount,omitempty"`
	CreatedAtTime  *Timestamp `ic:"created_at_time,omitempty" json:"created_at_time,omitempty"`
	Amount         Tokens     `ic:"amount" json:"amount"`
}

type TransferResult struct {
	Ok  *uint64        `ic:"Ok,variant" json:"Ok,omitempty"`
	Err *TransferError `ic:"Err,variant" json:"Err,omitempty"`
}

type TransferError struct {
	TxTooOld *struct {
		AllowedWindowNanos uint64 `ic:"allowed_window_nanos" json:"allowed_window_nanos"`
	} `ic:"TxTooOld,variant" json:"TxTooOld,omitempty"`
	BadFee *struct {
		ExpectedFee Tokens `ic:"expected_fee" json:"expected_fee"`
	} `ic:"BadFee,variant" json:"BadFee,omitempty"`
	TxDuplicate *struct {
		DuplicateOf uint64 `ic:"duplicate_of" json:"duplicate_of"`
	} `ic:"TxDuplicate,variant" json:"TxDuplicate,omitempty"`
	TxCreatedInFuture *idl.Null `ic:"TxCreatedInFuture,variant" json:"TxCreatedInFuture,omitempty"`
	InsufficientFunds *struct {
		Balance Tokens `ic:"balance" json:"balance"`
	} `ic:"InsufficientFunds,variant" json:"InsufficientFunds,omitempty"`
}

func (e *TransferError) Error() string {
	if e.TxTooOld != nil {
		return icperrors.TransactionTooOld()
	} else if e.BadFee != nil {
		return icperrors.BadFee(e.BadFee.ExpectedFee.E8s)
	} else if e.TxCreatedInFuture != nil {
		return icperrors.CreatedInFuture()
	} else if e.InsufficientFunds != nil {
		return icperrors.InsufficientFunds(e.InsufficientFunds.Balance.E8s)
	} else if e.TxDuplicate != nil {
		return icperrors.TransactionDuplicate(e.TxDuplicate.DuplicateOf)
	} else {
		return icperrors.Unknown()
	}
}

type GetAccountIdentifierTransactions struct {
	MaxResults        uint64   `ic:"max_results"`
	Start             *idl.Nat `ic:"start,omitempty"`
	AccountIdentifier string   `ic:"account_identifier"`
}

type AccountIdentifierTransactions struct {
	Balance      idl.Nat                     `ic:"balance"`
	Transactions []TransactionWithId[string] `ic:"transactions"`
}

type AccountIdentifierError struct {
	Message string `ic:"message"`
}

type GetAccountIdentifierTransactionsResult struct {
	Ok    *AccountIdentifierTransactions `ic:"Ok,omitempty"`
	Error *AccountIdentifierError        `ic:"Err"`
}
