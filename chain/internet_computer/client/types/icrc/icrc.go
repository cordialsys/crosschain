package icrc

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	icperrors "github.com/cordialsys/crosschain/chain/internet_computer/client/types/errors"
	"github.com/fxamacker/cbor/v2"
)

const (
	ICRCSubbaccountLen           = 32
	MethodBalanceOf              = "icrc1_balance_of"
	MethodFee                    = "icrc1_fee"
	MethodGetAccountTransactions = "get_account_transactions"
	MethodGetBlocks              = "icrc3_get_blocks"
	MethodGetIndexPrincipal      = "icrc106_get_index_principal"
	MethodName                   = "icrc1_name"
	MethodTransfer               = "icrc1_transfer"

	blockParentHash  = "phash"
	blockTimestamp   = "ts"
	blockTransaction = "tx"

	txCreatedAtTime     = "ts"
	txMemo              = "memo"
	txOperation         = "op"
	txFrom              = "from"
	txTo                = "to"
	txSpender           = "spender"
	txAmount            = "amt"
	txFee               = "fee"
	txExpectedAllowance = "expected_allowance"
	txExpiresAt         = "expires_at"
)

type Account struct {
	Owner      address.Principal `ic:"owner"`
	Subaccount *[]byte           `ic:"subaccount,omitempty"`
}

func (a *Account) MarshalCBOR() ([]byte, error) {
	arr := make([]any, 0)
	arr = append(arr, a.Owner)
	if a.Subaccount != nil {
		arr = append(arr, a.Subaccount)
	}

	return cbor.Marshal(arr)
}

func verifyChecksum(checksum string, principalBytes []byte, subaccount []byte) (bool, error) {
	var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)
	checksumBytes, err := encoding.DecodeString(strings.ToUpper(checksum))
	if err != nil {
		return false, err
	}
	expectedChecksum := binary.BigEndian.Uint32(checksumBytes)

	check := crc32.ChecksumIEEE(append(principalBytes, subaccount...))
	return expectedChecksum == check, nil
}

func DecodeAccount(addrs string) (Account, error) {
	parts := strings.Split(addrs, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return Account{}, fmt.Errorf("invalid address: %s", addrs)
	}

	if len(parts) == 1 {
		owner, err := address.Decode(addrs)
		if err != nil {
			return Account{}, fmt.Errorf("failed to decode principal: %w", err)
		}
		return Account{Owner: owner}, nil
	}

	principalParts := strings.Split(parts[0], "-")

	principalString := strings.Join(principalParts[:5], "-")
	owner, err := address.Decode(principalString)
	if err != nil {
		return Account{}, fmt.Errorf("failed to decode principal: %w", err)
	}

	subaccount, err := hex.DecodeString(parts[1])
	if err != nil {
		return Account{}, fmt.Errorf("failed to decode subaccount: %w", err)
	}
	if len(subaccount) != ICRCSubbaccountLen {
		diff := ICRCSubbaccountLen - len(subaccount)
		zeroes := make([]byte, diff)
		subaccount = append(zeroes, subaccount...)
	}

	checksum := principalParts[len(principalParts)-1]
	ok, err := verifyChecksum(checksum, owner.Raw, subaccount)
	if err != nil {
		return Account{}, fmt.Errorf("failed to verify checksum: %w", err)
	}
	if !ok {
		return Account{}, errors.New("invalid checksum")
	}

	return Account{Owner: owner, Subaccount: &subaccount}, nil
}

func (a Account) Encode() string {
	principal := a.Owner.Encode()
	if a.Subaccount != nil {
		subacc := hex.EncodeToString(*a.Subaccount)
		subacc = strings.TrimLeft(subacc, "0")

		checksumContent := []byte{}
		checksumContent = append(checksumContent, a.Owner.Raw...)
		checksumContent = append(checksumContent, *a.Subaccount...)
		checksum := crc32.ChecksumIEEE(checksumContent)
		var checksumBytes []byte
		checksumBytes = binary.BigEndian.AppendUint32(checksumBytes, checksum)

		var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)
		checksumStr := encoding.EncodeToString(checksumBytes)
		principal = fmt.Sprintf("%s-%s.%s", principal, strings.ToLower(checksumStr), subacc)
	}

	return principal
}

type GetBlocksRequest struct {
	Start  idl.Nat `ic:"start"`
	Length idl.Nat `ic:"length"`
}

type MapEntry struct {
	Key   string  `ic:"0"`
	Value Variant `ic:"1"`
}

type Variant struct {
	Nat   *idl.Nat    `ic:"Nat,omitempty,variant"`
	Blob  *[]byte     `ic:"Blob,omitempty,variant"`
	Map   *MapWrapper `ic:"Map,omitempty,variant"`
	Array *[]Variant  `ic:"Array,omitempty,variant"`
	Text  *string     `ic:"Text,omitempty,variant"`
}

type MapWrapper []MapEntry

func (m MapWrapper) GetValue(key string, value any) (bool, error) {
	for _, e := range []MapEntry(m) {
		if e.Key == key {
			switch value.(type) {
			case *idl.Nat:
				value = e.Value.Nat
			case *[]byte:
				value = e.Value.Blob
			case *MapWrapper:
				value = e.Value.Map
			case *[]Variant:
				value = e.Value.Array
			case *string:
				value = e.Value.Text
			default:
				return false, errors.New("unsupported type")
			}

			return true, nil
		}
	}

	return false, nil
}

type Block struct {
	Map *MapWrapper `ic:"Map,omitempty,variant"`
}

var _ types.Block = &Block{}

func (b Block) ParentHash() (*[]byte, error) {
	if b.Map == nil {
		return nil, errors.New("invalid block")
	}

	ph := []byte{}
	_, err := b.Map.GetValue(blockParentHash, &ph)
	if err != nil {
		return nil, fmt.Errorf("failed to get phash: %w", err)
	}
	return &ph, nil
}

func (b Block) RawTransaction() (MapWrapper, error) {
	if b.Map == nil {
		return MapWrapper{}, errors.New("invalid block")
	}

	var tx Variant
	_, err := b.Map.GetValue(blockTransaction, &tx)
	if err != nil {
		return MapWrapper{}, fmt.Errorf("failed to get transaction: %w", err)
	}

	if tx.Map == nil {
		return MapWrapper{}, fmt.Errorf("transaction should be a Variant.Map %w", err)
	}

	return *tx.Map, nil
}

func (b Block) Hash() (string, error) {
	phash, err := b.ParentHash()
	if err != nil {
		return "", fmt.Errorf("failed to get parent hash: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(*phash)

	timestamp, err := b.Timestamp()
	if err != nil {
		return "", fmt.Errorf("failed to get timestamp: %w", err)
	}
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, timestamp)
	hasher.Write(timestampBytes)

	tx, err := b.FlattenedTransaction()
	if err != nil {
		return "", fmt.Errorf("failed to get flattened transaction: %w", err)
	}

	txBytes, err := candid.Marshal([]interface{}{tx})
	if err != nil {
		return "", fmt.Errorf("failed to marshal tx: %w", err)
	}
	hasher.Write(txBytes)

	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash), nil
}

func (b Block) Timestamp() (uint64, error) {
	if b.Map == nil {
		return 0, errors.New("invalid block")
	}

	var timestamp idl.Nat
	ok, err := b.Map.GetValue(blockTimestamp, &timestamp)
	if err != nil {
		return 0, fmt.Errorf("failed to get block timestamp: %w", err)
	}

	if !ok {
		return 0, errors.New("missing block timestamp")
	}
	fmt.Printf("ts: %v\n", timestamp)

	return timestamp.BigInt().Uint64(), nil
}

func (b Block) FlattenedTransaction() (FlattenedTransaction, error) {
	tx, err := b.RawTransaction()
	if err != nil {
		return FlattenedTransaction{}, fmt.Errorf("failed to get tx: %w", err)
	}

	var createdAt idl.Nat
	ok, err := tx.GetValue(txCreatedAtTime, &createdAt)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var ts *uint64
	if ok {
		*ts = createdAt.BigInt().Uint64()
	}

	var memo []byte
	_, err = tx.GetValue(txMemo, &memo)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var memoP *[]byte
	if memo != nil {
		memoP = &memo
	}

	var operation string
	ok, err = tx.GetValue(txOperation, &operation)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	if !ok {
		return FlattenedTransaction{}, errors.New("missing operation")
	}

	var from []byte
	_, err = tx.GetValue(txFrom, &from)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var fromAcc *Account
	if from != nil {
		var account Account
		err := cbor.Unmarshal(from, account)
		if err != nil {
			return FlattenedTransaction{}, fmt.Errorf("failed to unmarshal 'from' account: %w", err)
		}
		fromAcc = &account
	}

	var to []byte
	_, err = tx.GetValue(txTo, &to)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var toAcc *Account
	if to != nil {
		var account Account
		err := cbor.Unmarshal(to, account)
		if err != nil {
			return FlattenedTransaction{}, fmt.Errorf("failed to unmarshal 'to' account: %w", err)
		}
		toAcc = &account
	}

	var spender []byte
	_, err = tx.GetValue(txSpender, &spender)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var spenderAcc *Account
	if spender != nil {
		var account Account
		err := cbor.Unmarshal(spender, account)
		if err != nil {
			return FlattenedTransaction{}, fmt.Errorf("failed to unmarshal 'to' account: %w", err)
		}
		spenderAcc = &account
	}

	var amount idl.Nat
	_, err = tx.GetValue(txAmount, &amount)
	if err != nil {
		return FlattenedTransaction{}, err
	}

	var fee idl.Nat
	ok, err = tx.GetValue(txFee, &fee)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var feeP *uint64
	if ok {
		*feeP = fee.BigInt().Uint64()
	}

	var expectedAllowance idl.Nat
	ok, err = tx.GetValue(txExpectedAllowance, &expectedAllowance)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var expectedAllowanceP *uint64
	if ok {
		*expectedAllowanceP = expectedAllowance.BigInt().Uint64()
	}

	var expiresAt idl.Nat
	ok, err = tx.GetValue(txExpectedAllowance, &expiresAt)
	if err != nil {
		return FlattenedTransaction{}, err
	}
	var expiresAtP *uint64
	if ok {
		*expiresAtP = expiresAt.BigInt().Uint64()
	}

	return FlattenedTransaction{
		CreatedAtTime:     ts,
		Memo:              memoP,
		Op:                operation,
		From:              fromAcc,
		To:                toAcc,
		Spender:           spenderAcc,
		Amount:            amount.BigInt().Uint64(),
		Fee:               feeP,
		ExpectedAllowance: expectedAllowanceP,
		ExpiresAt:         expiresAtP,
	}, nil
}

func (b Block) TxHash() (string, error) {
	ft, err := b.FlattenedTransaction()
	if err != nil {
		return "", fmt.Errorf("failed to get flattened transaction: %w", err)
	}
	return ft.Hash()
}

func (b Block) Transaction() (types.Transaction, error) {
	ft, err := b.FlattenedTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to get flattened transaction: %w", err)
	}

	return ft.ToTransaction(), nil
}

type BlockWithId struct {
	Id    idl.Nat `ic:"id"`
	Block Block   `ic:"block,variant"`
}

type ArchivedBlocks struct {
	Args     []GetBlocksRequest `ic:"args"`
	Callback idl.Function       `ic:"callback"`
}

type GetBlocksResponse struct {
	// Total number of blocks in the ledger
	LogLength      idl.Nat          `ic:"log_length"`
	Blocks         []BlockWithId    `ic:"blocks"`
	ArchivedBlocks []ArchivedBlocks `ic:"archived_blocks"`
}

type GetIndexPrincipalResponse struct {
	Ok  *address.Principal      `ic:"Ok,omitempty"`
	Err *GetIndexPrincipalError `ic:"Err,omitempty"`
}

type GetIndexPrincipalError struct {
	GenericError         *GenericError `ic:"GenericError,omitempty"`
	IndexPrincipalNotSet *struct{}     `ic:"IndexPrincipalNotSet,omitempty"`
}

func (e *GetIndexPrincipalError) Error() string {
	if e.GenericError != nil {
		return fmt.Sprintf("generic transaction error, code: %d, message: %s", e.GenericError.ErrorCode.BigInt().Uint64(), e.GenericError.Message)
	} else if e.IndexPrincipalNotSet != nil {
		return "index principal is not set"
	} else {
		return "unknown error"
	}
}

type TransferArgs struct {
	FromSubaccount *[]byte `ic:"from_subaccount,omitempty"`
	To             Account `ic:"to"`
	Amount         idl.Nat `ic:"amount"`
	Fee            idl.Nat `ic:"fee,omitempty"`
	Memo           *[]byte `ic:"memo,omitempty"`
	CreatedAtTime  *uint64 `ic:"created_at_time,omitempty"`
}

type TransferResult struct {
	Ok  *idl.Nat       `ic:"Ok,variant" json:"Ok,omitempty"`
	Err *TransferError `ic:"Err,variant" json:"Err,omitempty"`
}

type TransferError struct {
	BadFee            *BadFeeError            `ic:"BadFee,variant"`
	BadBurn           *BadBurnError           `ic:"BadBurn,variant"`
	InsufficientFunds *InsufficientFundsError `ic:"InsufficientFunds,variant"`
	TooOld            *struct{}               `ic:"TooOld,variant"`
	CreatedInFuture   *CreatedInFutureError   `ic:"CreatedInFuture,variant"`
	Duplicate         *DuplicateError         `ic:"Duplicate,variant"`
	GenericError      *GenericError           `ic:"GenericError,variant"`
}

type BadFeeError struct {
	ExpectedFee idl.Nat `ic:"expected_fee"`
}

type BadBurnError struct {
	MinBurnAmount idl.Nat `ic:"min_burn_amount"`
}

type InsufficientFundsError struct {
	Balance idl.Nat `ic:"balance"`
}

type CreatedInFutureError struct {
	LedgerTime uint64 `ic:"ledger_time"`
}

type DuplicateError struct {
	DuplicateOf idl.Nat `ic:"duplicate_of"`
}

type GenericError struct {
	ErrorCode idl.Nat `ic:"error_code"`
	Message   string  `ic:"message"`
}

func (e *TransferError) Error() string {
	if e.BadFee != nil {
		return icperrors.BadFee(e.BadFee.ExpectedFee.BigInt().Uint64())
	} else if e.BadBurn != nil {
		return icperrors.BadBurn(e.BadBurn.MinBurnAmount.BigInt().Uint64())
	} else if e.InsufficientFunds != nil {
		return icperrors.InsufficientFunds(e.BadBurn.MinBurnAmount.BigInt().Uint64())
	} else if e.TooOld != nil {
		return icperrors.TransactionTooOld()
	} else if e.CreatedInFuture != nil {
		return icperrors.CreatedInFuture()
	} else if e.Duplicate != nil {
		return icperrors.TransactionDuplicate(e.Duplicate.DuplicateOf.BigInt().Uint64())
	} else if e.GenericError != nil {
		return icperrors.Generic(e.GenericError.ErrorCode.BigInt().Uint64(), e.GenericError.Message)
	} else {
		return icperrors.Unknown()
	}
}

type GetAccountTransactionsArgs struct {
	MaxResults idl.Nat  `ic:"max_results"`
	Start      *idl.Nat `ic:"start,omitempty"`
	Account    Account  `ic:"account"`
}

type GetAccountTransactionsResponse struct {
	Ok    *AccountTransactions `ic:"Ok,omitempty"`
	Error *struct {
		Message string `ic:"message"`
	} `ic:"Error,omitempty"`
}

type AccountTransactions struct {
	Balance      idl.Nat             `ic:"balance"`
	Transactions []TransactionWithId `ic:"transactions"`
	OldestTxId   *idl.Nat            `ic:"oldest_tx_id,omitempty"`
}

type TransactionWithId struct {
	Id          idl.Nat     `ic:"id"`
	Transaction Transaction `ic:"transaction"`
}

type Transaction struct {
	Kind      string    `ic:"kind"`
	Burn      *Burn     `ic:"burn,omitempty"`
	Mint      *Mint     `ic:"mint,omitempty"`
	Approve   *Approve  `ic:"approve,omitempty"`
	Timestamp idl.Nat   `ic:"timestamp"`
	Transfer  *Transfer `ic:"transfer,omitempty"`
}

var _ types.Transaction = &Transaction{}

func (t Transaction) Hash() (string, error) {
	return t.ToFlattened().Hash()
}

func (t Transaction) CreatedAtTime() *uint64 {
	if t.Burn != nil {
		ts := new(uint64)
		*ts = t.Burn.CreatedAtTime.BigInt().Uint64()
		return ts
	} else if t.Mint != nil {
		ts := new(uint64)
		*ts = t.Mint.CreatedAtTime.BigInt().Uint64()
		return ts
	} else if t.Approve != nil {
		ts := new(uint64)
		*ts = t.Approve.CreatedAtTime.BigInt().Uint64()
		return ts
	} else if t.Transfer != nil && t.Transfer.CreatedAtTime != nil {
		ts := new(uint64)
		*ts = t.Transfer.CreatedAtTime.BigInt().Uint64()
		return ts
	}
	return nil
}

func (t Transaction) TxTime() uint64 {
	return t.Timestamp.BigInt().Uint64()
}

func (t Transaction) Memo() string {
	if t.Burn != nil && t.Burn.Memo != nil {
		return hex.EncodeToString(*t.Burn.Memo)
	} else if t.Mint != nil && t.Mint.Memo != nil {
		return hex.EncodeToString(*t.Mint.Memo)
	} else if t.Approve != nil && t.Approve.Memo != nil {
		return hex.EncodeToString(*t.Approve.Memo)
	} else if t.Transfer != nil && t.Transfer.Memo != nil {
		return hex.EncodeToString(*t.Transfer.Memo)
	}
	return ""
}

func (t Transaction) Op() string {
	if t.Burn != nil {
		return "burn"
	} else if t.Mint != nil {
		return "mint"
	} else if t.Approve != nil {
		return "approve"
	} else if t.Transfer != nil {
		return "transfer"
	}
	return ""
}

func (t Transaction) From() *Account {
	if t.Burn != nil {
		return &t.Burn.From
	} else if t.Mint != nil {
		return nil
	} else if t.Approve != nil {
		return &t.Approve.From
	} else if t.Transfer != nil {
		return &t.Transfer.From
	}
	return nil
}

func (t Transaction) SourceAddress() string {
	acc := t.From()
	if acc == nil {
		return ""
	}

	return acc.Encode()
}

func (t Transaction) To() *Account {
	if t.Burn != nil {
		return nil
	} else if t.Mint != nil {
		return &t.Mint.To
	} else if t.Approve != nil {
		return nil
	} else if t.Transfer != nil {
		return &t.Transfer.To
	}
	return nil
}

func (t Transaction) DestinationAddress() string {
	acc := t.To()
	if acc == nil {
		return ""
	}

	return acc.Encode()
}

func (t Transaction) Spender() *Account {
	if t.Burn != nil {
		return t.Burn.Spender
	} else if t.Mint != nil {
		return nil
	} else if t.Approve != nil {
		return &t.Approve.Spender
	} else if t.Transfer != nil {
		return t.Transfer.Spender
	}
	return nil
}

func (t Transaction) Amount() (uint64, error) {
	if t.Burn != nil {
		return t.Burn.Amount.BigInt().Uint64(), nil
	} else if t.Mint != nil {
		return t.Mint.Amount.BigInt().Uint64(), nil
	} else if t.Approve != nil {
		return t.Approve.Amount.BigInt().Uint64(), nil
	} else if t.Transfer != nil {
		return t.Transfer.Amount.BigInt().Uint64(), nil
	}
	return 0, errors.New("invalid transaction, missing operation")
}

func (t Transaction) RawFee() *uint64 {
	if t.Burn != nil {
		return nil
	} else if t.Mint != nil {
		return nil
	} else if t.Approve != nil {
		fee := new(uint64)
		*fee = t.Approve.Fee.BigInt().Uint64()
		return fee
	} else if t.Transfer != nil {
		fee := new(uint64)
		*fee = t.Transfer.Fee.BigInt().Uint64()
		return fee
	}

	return nil
}

func (t Transaction) Fee() uint64 {
	rawFee := t.RawFee()
	if rawFee == nil {
		return 0
	}
	return *rawFee
}

func (t Transaction) ExpectedAllowance() *uint64 {
	if t.Approve != nil {
		ea := new(uint64)
		*ea = t.Approve.ExpectedAllowance.BigInt().Uint64()
		return ea
	}

	return nil
}

func (t Transaction) ExpiresAt() *uint64 {
	if t.Approve != nil {
		ea := new(uint64)
		*ea = t.Approve.ExpiresAt.BigInt().Uint64()
		return ea
	}

	return nil
}

func (t Transaction) ToFlattened() FlattenedTransaction {
	var memoBytes *[]byte
	memo := t.Memo()
	if len(memo) > 0 {
		mB, _ := hex.DecodeString(memo)
		memoBytes = &mB
	}

	amnt, _ := t.Amount()
	return FlattenedTransaction{
		CreatedAtTime:     t.CreatedAtTime(),
		Memo:              memoBytes,
		Op:                t.Op(),
		From:              t.From(),
		To:                t.To(),
		Spender:           t.Spender(),
		Amount:            amnt,
		Fee:               t.RawFee(),
		ExpectedAllowance: t.ExpectedAllowance(),
		ExpiresAt:         t.ExpiresAt(),
	}
}

type FlattenedTransaction struct {
	CreatedAtTime     *uint64  `cbor:"ts,omitempty"`
	Memo              *[]byte  `cbor:"memo,omitempty"`
	Op                string   `cbor:"op"`
	From              *Account `cbor:"from,omitempty"`
	To                *Account `cbor:"to,omitempty"`
	Spender           *Account `cbor:"spender,omitempty"`
	Amount            uint64   `cbor:"amt"`
	Fee               *uint64  `cbor:"fee,omitempty"`
	ExpectedAllowance *uint64  `cbor:"expected_allowance,omitempty"`
	ExpiresAt         *uint64  `cbor:"expires_at,omitempty"`
}

func (t FlattenedTransaction) ToTransaction() Transaction {
	transaction := Transaction{
		Kind:     t.Op,
		Burn:     nil,
		Mint:     nil,
		Approve:  nil,
		Transfer: nil,
	}
	var createdAtTime *idl.Nat
	if t.CreatedAtTime != nil {
		createdAtTime = new(idl.Nat)
		*createdAtTime = idl.NewNat(*t.CreatedAtTime)
	}

	amount := idl.NewNat(t.Amount)
	var fee *idl.Nat
	if t.Fee != nil {
		fee = new(idl.Nat)
		*fee = idl.NewNat(*t.Fee)
	}

	var mint *Mint
	// Only 'Mint' operation is missing 'From'
	if t.From == nil {
		mint = new(Mint)
		mint.To = *t.From
		mint.Memo = t.Memo
		mint.CreatedAtTime = createdAtTime
		mint.Amount = amount
		transaction.Mint = mint

		return transaction
	}

	// Both 'to' and 'from' is used only for 'Transfer'
	if t.To != nil {
		transfer := new(Transfer)
		transfer.To = *t.To
		transfer.From = *t.From
		transfer.Fee = fee
		transfer.Memo = t.Memo
		transfer.CreatedAtTime = createdAtTime
		transfer.Amount = amount
		transfer.Spender = t.Spender
		transaction.Transfer = transfer

		return transaction
	}

	// 'Burn' doesn't have fee
	if t.Fee == nil {
		burn := new(Burn)
		burn.From = *t.From
		burn.Memo = t.Memo
		burn.CreatedAtTime = createdAtTime
		burn.Amount = amount
		burn.Spender = t.Spender
		transaction.Burn = burn

		return transaction
	}

	// Only 'Approve' is left
	approve := new(Approve)
	approve.Fee = fee
	approve.From = *t.From
	approve.Memo = t.Memo
	approve.CreatedAtTime = createdAtTime
	approve.Amount = amount
	var expectedAllowance *idl.Nat
	if t.ExpectedAllowance != nil {
		expectedAllowance = new(idl.Nat)
		*expectedAllowance = idl.NewNat(*t.ExpectedAllowance)
	}
	approve.ExpectedAllowance = expectedAllowance
	var expiresAt *idl.Nat
	if t.ExpiresAt != nil {
		expiresAt = new(idl.Nat)
		*expiresAt = idl.NewNat(*t.ExpiresAt)
	}
	approve.ExpiresAt = expiresAt
	approve.Spender = *t.Spender
	transaction.Approve = approve

	return transaction
}

// TODO: Use proper icrc3 hash implementation when indexing by hash is out.
func (t FlattenedTransaction) Hash() (string, error) {
	cborData, err := cbor.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction hash: %w", err)
	}
	hasher := sha256.New()
	hasher.Write(cborData)
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash), nil
}

type Transfer struct {
	To            Account  `ic:"to"`
	From          Account  `ic:"from"`
	Fee           *idl.Nat `ic:"fee,omitempty"`
	Memo          *[]byte  `ic:"memo,omitempty"`
	CreatedAtTime *idl.Nat `ic:"created_at_time,omitempty"`
	Amount        idl.Nat  `ic:"amount"`
	Spender       *Account `ic:"spender"`
}

type Burn struct {
	From          Account  `ic:"from"`
	Memo          *[]byte  `ic:"memo"`
	CreatedAtTime *idl.Nat `ic:"created_at_time,omitempty"`
	Amount        idl.Nat  `ic:"amount"`
	Spender       *Account `ic:"spender"`
}

type Approve struct {
	Fee               *idl.Nat `ic:"fee,omitempty"`
	From              Account  `ic:"from"`
	Memo              *[]byte  `ic:"memo,omitempty"`
	CreatedAtTime     *idl.Nat `ic:"created_at_time,omitempty"`
	Amount            idl.Nat  `ic:"amount"`
	ExpectedAllowance *idl.Nat `ic:"expected_allowance,omitempty"`
	ExpiresAt         *idl.Nat
	Spender           Account `ic:"spender,omitempty"`
}

type Mint struct {
	To            Account  `ic:"to"`
	Memo          *[]byte  `ic:"memo,omitempty"`
	CreatedAtTime *idl.Nat `ic:"created_at_time,omitempty"`
	Amount        idl.Nat  `ic:"amount"`
}
