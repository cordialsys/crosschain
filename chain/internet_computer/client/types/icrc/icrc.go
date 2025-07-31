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
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
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

type GetBlocksResponse struct {
	// Total number of blocks in the ledger
	LogLength      idl.Nat `ic:"log_length"`
	Blocks         []any   `ic:"blocks"`
	ArchivedBlocks []any   `ic:"archived_blocks"`
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

func (t Transaction) Memo() *[]byte {
	if t.Burn != nil {
		return t.Burn.Memo
	} else if t.Mint != nil {
		return t.Mint.Memo
	} else if t.Approve != nil {
		return t.Approve.Memo
	} else if t.Transfer != nil {
		return t.Transfer.Memo
	}
	return nil
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

func (t Transaction) Amount() uint64 {
	if t.Burn != nil {
		return t.Burn.Amount.BigInt().Uint64()
	} else if t.Mint != nil {
		return t.Mint.Amount.BigInt().Uint64()
	} else if t.Approve != nil {
		return t.Approve.Amount.BigInt().Uint64()
	} else if t.Transfer != nil {
		return t.Transfer.Amount.BigInt().Uint64()
	}
	return 0
}

func (t Transaction) Fee() *uint64 {
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
	return FlattenedTransaction{
		CreatedAtTime:     t.CreatedAtTime(),
		Memo:              t.Memo(),
		Op:                t.Op(),
		From:              t.From(),
		To:                t.To(),
		Spender:           t.Spender(),
		Amount:            t.Amount(),
		Fee:               t.Fee(),
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
