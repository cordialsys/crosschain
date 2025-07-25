package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/leb128"
	"github.com/fxamacker/cbor/v2"
)

type RequestType string

const (
	RequestTypeQuery     = RequestType("query")
	RequestTypeCall      = RequestType("call")
	MethodAccountBalance = "account_balance"
	MethodQueryBlocks    = "query_blocks"
	MethodTransfer       = "transfer"
	MethodICRCTransfer   = "icrc1_transfer"
	MethodICRCBalanceOf  = "icrc1_balance_of"
	MethodICRCFee        = "icrc1_fee"
	ICRCSubbaccountLen   = 32
)

var (
	typeKey            = sha256.Sum256([]byte("request_type"))
	canisterIDKey      = sha256.Sum256([]byte("canister_id"))
	nonceKey           = sha256.Sum256([]byte("nonce"))
	methodNameKey      = sha256.Sum256([]byte("method_name"))
	argumentsKey       = sha256.Sum256([]byte("arg"))
	ingressExpiryKey   = sha256.Sum256([]byte("ingress_expiry"))
	senderKey          = sha256.Sum256([]byte("sender"))
	pathsKey           = sha256.Sum256([]byte("paths"))
	IcpLedgerPrincipal = address.MustDecode("ryjl3-tyaaa-aaaaa-aaaba-cai")
)

// RequestID is the request ID.
type RequestID [32]byte

func NewRequestID(req Request) RequestID {
	var hashes [][]byte
	if len(req.Type) != 0 {
		typeHash := sha256.Sum256([]byte(req.Type))
		hashes = append(hashes, append(typeKey[:], typeHash[:]...))
	}
	// NOTE: the canister ID may be the empty slice. The empty slice doesn't mean it's not
	// set, it means it's the management canister (aaaaa-aa).
	if req.CanisterID.Raw != nil {
		canisterIDHash := sha256.Sum256(req.CanisterID.Raw)
		hashes = append(hashes, append(canisterIDKey[:], canisterIDHash[:]...))
	}
	if len(req.MethodName) != 0 {
		methodNameHash := sha256.Sum256([]byte(req.MethodName))
		hashes = append(hashes, append(methodNameKey[:], methodNameHash[:]...))
	}
	if len(req.Arguments) != 0 {
		argumentsHash := sha256.Sum256(req.Arguments)
		hashes = append(hashes, append(argumentsKey[:], argumentsHash[:]...))
	}
	principal, err := req.Sender.Principal()
	if err != nil {
		return RequestID{}
	}
	if len(principal.Raw) != 0 {
		senderHash := sha256.Sum256(principal.Raw)
		hashes = append(hashes, append(senderKey[:], senderHash[:]...))
	}
	if req.IngressExpiry != 0 {
		bi := big.NewInt(int64(req.IngressExpiry))
		e, _ := leb128.EncodeUnsigned(bi)
		ingressExpiryHash := sha256.Sum256(e)
		hashes = append(hashes, append(ingressExpiryKey[:], ingressExpiryHash[:]...))
	}
	if len(req.Nonce) != 0 {
		nonceHash := sha256.Sum256(req.Nonce)
		hashes = append(hashes, append(nonceKey[:], nonceHash[:]...))
	}
	if req.Paths != nil {
		pathsHash := hashPaths(req.Paths)
		hashes = append(hashes, append(pathsKey[:], pathsHash[:]...))
	}
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i], hashes[j]) == -1
	})
	return sha256.Sum256(bytes.Join(hashes, nil))
}

func (r RequestID) PrepareForSign() []byte {
	message := append(
		// \x0Aic-request
		[]byte{0x0a, 0x69, 0x63, 0x2d, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74},
		r[:]...,
	)
	return message
}

type Request struct {
	Type          RequestType
	Sender        address.Ed25519Identity
	Nonce         []byte
	IngressExpiry uint64
	CanisterID    address.Principal
	MethodName    string
	Arguments     []byte
	Paths         [][][]byte
	Signature     []byte
}

func (r Request) RequestID() RequestID {
	return NewRequestID(r)
}

// Request wrapper that is sent to the canister.
// SenderPubKey and SenderSig can be empty if using an anonymous identity.
type Envelope struct {
	Content      Request `cbor:"content,omitempty"`
	SenderPubKey []byte  `cbor:"sender_pubkey,omitempty"`
	SenderSig    []byte  `cbor:"sender_sig,omitempty"`
}

func (r Request) Sign(signature []byte) ([]byte, error) {
	senderPk, err := r.Sender.DerPublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create request envelope: %v", err)
	}
	data, err := cbor.Marshal(Envelope{
		Content:      r,
		SenderPubKey: senderPk,
		SenderSig:    signature,
	})

	return data, err
}

func (r *Request) MarshalCBOR() ([]byte, error) {
	m := make(map[string]any)
	if len(r.Type) != 0 {
		m["request_type"] = r.Type
	}
	if r.CanisterID.Raw != nil {
		m["canister_id"] = []byte(r.CanisterID.Raw)
	}
	if len(r.MethodName) != 0 {
		m["method_name"] = r.MethodName
	}
	if r.Arguments != nil {
		m["arg"] = r.Arguments
	} else {
		m["arg"] = []byte{}
	}

	principal, err := r.Sender.Principal()
	if err != nil {
		return nil, fmt.Errorf("failed to get sender principal: %v", err)
	}

	if len(principal.Raw) != 0 {
		m["sender"] = []byte(principal.Raw)
	}
	if r.IngressExpiry != 0 {
		m["ingress_expiry"] = r.IngressExpiry
	}
	if len(r.Nonce) != 0 {
		m["nonce"] = r.Nonce
	}
	if r.Paths != nil {
		m["paths"] = r.Paths
	}
	return cbor.Marshal(m)

}

func hashPaths(paths [][][]byte) [32]byte {
	var hash []byte
	for _, path := range paths {
		var rawPathHash []byte
		for _, p := range path {
			pathBytes := sha256.Sum256(p)
			rawPathHash = append(rawPathHash, pathBytes[:]...)
		}
		pathHash := sha256.Sum256(rawPathHash)
		hash = append(hash, pathHash[:]...)
	}
	return sha256.Sum256(hash)
}

// Response is the response from the agent.
type Response struct {
	Status     string              `cbor:"status"`
	Reply      cbor.RawMessage     `cbor:"reply"`
	RejectCode uint64              `cbor:"reject_code"`
	RejectMsg  string              `cbor:"reject_message"`
	ErrorCode  string              `cbor:"error_code"`
	Signatures []ResponseSignature `cbor:"signatures"`
}

type ResponseSignature struct {
	Timestamp int64             `cbor:"timestamp"`
	Signature []byte            `cbor:"signature"`
	Identity  address.Principal `cbor:"identity"`
}

type PreprocessingError struct {
	// The reject code.
	RejectCode uint64 `cbor:"reject_code"`
	// A textual diagnostic message.
	Message string `cbor:"reject_message"`
	// An optional implementation-specific textual error code.
	ErrorCode string `cbor:"error_code"`
}

func (e PreprocessingError) Error() string {
	return fmt.Sprintf("(%d) %s: %s", e.RejectCode, e.Message, e.ErrorCode)
}

type BalanceArgs struct {
	Account []byte `ic:"account"`
}

type IcpBalance struct {
	E8S uint64 `ic:"e8s"`
}

type ICRC1Account struct {
	Owner      address.Principal `ic:"owner"`
	Subaccount *[]byte           `ic:"subaccount,omitempty"`
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

func DecodeICRC1Account(addrs string) (ICRC1Account, error) {
	parts := strings.Split(addrs, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return ICRC1Account{}, fmt.Errorf("invalid address: %s", addrs)
	}

	if len(parts) == 1 {
		owner, err := address.Decode(addrs)
		if err != nil {
			return ICRC1Account{}, fmt.Errorf("failed to decode principal: %w", err)
		}
		return ICRC1Account{Owner: owner}, nil
	}

	principalParts := strings.Split(parts[0], "-")

	principalString := strings.Join(principalParts[:5], "-")
	owner, err := address.Decode(principalString)
	if err != nil {
		return ICRC1Account{}, fmt.Errorf("failed to decode principal: %w", err)
	}

	subaccount, err := hex.DecodeString(parts[1])
	if err != nil {
		return ICRC1Account{}, fmt.Errorf("failed to decode subaccount: %w", err)
	}
	if len(subaccount) != ICRCSubbaccountLen {
		diff := ICRCSubbaccountLen - len(subaccount)
		zeroes := make([]byte, diff)
		subaccount = append(zeroes, subaccount...)
	}

	checksum := principalParts[len(principalParts)-1]
	ok, err := verifyChecksum(checksum, owner.Raw, subaccount)
	if err != nil {
		return ICRC1Account{}, fmt.Errorf("failed to verify checksum: %w", err)
	}
	if !ok {
		return ICRC1Account{}, errors.New("invalid checksum")
	}

	return ICRC1Account{Owner: owner, Subaccount: &subaccount}, nil
}

func (a ICRC1Account) Encode() string {
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

type Mint struct {
	To     []byte `ic:"to" json:"to"`
	Amount Tokens `ic:"amount" json:"amount"`
}

func (m *Mint) MarshalCBOR() ([]byte, error) {
	type MintCBOR struct {
		To     string `cbor:"0,keyasint"`
		Amount Tokens `cbor:"1,keyasint"`
	}
	mintCBOR := MintCBOR{
		To:     hex.EncodeToString(m.To),
		Amount: m.Amount,
	}

	return cbor.Marshal(mintCBOR)
}

type Burn struct {
	From    []byte  `ic:"from" json:"from"`
	Amount  Tokens  `ic:"amount" json:"amount"`
	Spender *[]byte `ic:"spender,omitempty" json:"spender,omitempty"`
}

func (b *Burn) MarshalCBOR() ([]byte, error) {
	type BurnCBOR struct {
		From    string `cbor:"0,keyasint"`
		Amount  Tokens `cbor:"1,keyasint"`
		Spender string `cbor:"2,keyasint,omitempty"`
	}
	spender := ""
	if b.Spender != nil {
		spender = hex.EncodeToString(*b.Spender)
	}

	burnCBOR := BurnCBOR{
		From:    hex.EncodeToString(b.From),
		Amount:  b.Amount,
		Spender: spender,
	}

	return cbor.Marshal(burnCBOR)
}

type Approve struct {
	From              []byte     `ic:"from" json:"from"`
	Spender           []byte     `ic:"spender" json:"spender"`
	Allowance         Tokens     `ic:"allowance" json:"allowance"`
	AllowanceE8s      idl.Int    `ic:"allowance_e8s" json:"allowance_e8s"`
	ExpectedAllowance *Tokens    `ic:"expected_allowance,omitempty" json:"expected_allowance,omitempty"`
	ExpiresAt         *Timestamp `ic:"expires_at,omitempty" json:"expires_at,omitempty"`
	Fee               Tokens     `ic:"fee" json:"fee"`
}

func (a *Approve) MarshalCBOR() ([]byte, error) {
	type ApproveCBOR struct {
		From              string     `cbor:"0,keyasint"`
		To                string     `cbor:"1,keyasint"`
		Allowance         Tokens     `cbor:"2,keyasint"`
		ExpectedAllowance *Tokens    `cbor:"3,keyasint"`
		ExpiresAt         *Timestamp `cbor:"4,keyasint"`
		Fee               Tokens     `cbor:"5,keyasint"`
	}
	approveCBOR := ApproveCBOR{
		From:              hex.EncodeToString(a.From),
		To:                hex.EncodeToString(a.Spender),
		Allowance:         a.Allowance,
		ExpectedAllowance: a.ExpectedAllowance,
		ExpiresAt:         a.ExpiresAt,
		Fee:               a.Fee,
	}

	return cbor.Marshal(approveCBOR)
}

type Transfer struct {
	From    []byte   `ic:"from" json:"from"`
	To      []byte   `ic:"to" json:"to"`
	Amount  Tokens   `ic:"amount" json:"amount"`
	Fee     Tokens   `ic:"fee" json:"fee"`
	Spender *[]uint8 `ic:"spender,omitempty" json:"spender"`
}

func (t *Transfer) MarshalCBOR() ([]byte, error) {
	type TransferCBOR struct {
		From    string `ic:"from" cbor:"0,keyasint"`
		To      string `ic:"to" cbor:"1,keyasint"`
		Amount  Tokens `ic:"amount" cbor:"2,keyasint"`
		Fee     Tokens `ic:"fee" cbor:"3,keyasint"`
		Spender string `ic:"spender,omitempty" cbor:"4,keyasint,omitempty"`
	}

	spender := ""
	if t.Spender != nil {
		spender = hex.EncodeToString(*t.Spender)
	}
	transferCbor := TransferCBOR{
		From:    hex.EncodeToString(t.From),
		To:      hex.EncodeToString(t.To),
		Amount:  t.Amount,
		Fee:     t.Fee,
		Spender: spender,
	}

	return cbor.Marshal(transferCbor)
}

type Operation struct {
	Burn     *Burn     `ic:"Burn,variant" cbor:"0,keyasint,omitempty"`
	Mint     *Mint     `ic:"Mint,variant" cbor:"1,keyasint,omitempty"`
	Transfer *Transfer `ic:"Transfer,variant" cbor:"2,keyasint,omitempty"`
	Approve  *Approve  `ic:"Approve,variant" cbor:"3,keyasint,omitempty"`
}

type Transaction struct {
	Operation     *Operation `ic:"operation,omitempty" cbor:"0,keyasint"`
	Memo          uint64     `ic:"memo" cbor:"1,keyasint"`
	CreatedAtTime Timestamp  `ic:"created_at_time" cbor:"2,keyasint,omitempty"`
	Icrc1Memo     *[]byte    `ic:"icrc1_memo,omitempty" cbor:"3,keyasint,omitempty"`
}

func (tx Transaction) Hash() (string, error) {
	cborData, err := cbor.Marshal(tx)
	if err != nil {
		return "", fmt.Errorf("failed to CBOR marshal transaction: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(cborData)
	hash := hasher.Sum(nil)

	return hex.EncodeToString(hash), nil
}

func (tx Transaction) Amount() Tokens {
	if tx.Operation.Transfer != nil {
		return tx.Operation.Transfer.Amount
	} else if tx.Operation.Burn != nil {
		return tx.Operation.Burn.Amount
	} else if tx.Operation.Mint != nil {
		return tx.Operation.Mint.Amount
	} else if tx.Operation.Approve != nil {
		return tx.Operation.Approve.Allowance
	}

	return Tokens{
		E8s: 0,
	}
}

func (tx Transaction) Fee() Tokens {
	if tx.Operation.Transfer != nil {
		return tx.Operation.Transfer.Fee
	} else if tx.Operation.Burn != nil {
		return Tokens{E8s: 0}
	} else if tx.Operation.Mint != nil {
		return Tokens{E8s: 0}
	} else if tx.Operation.Approve != nil {
		return tx.Operation.Approve.Fee
	}

	return Tokens{
		E8s: 0,
	}
}

func (tx Transaction) SourceAddress() string {
	if tx.Operation.Transfer != nil {
		return hex.EncodeToString(tx.Operation.Transfer.From)
	} else if tx.Operation.Burn != nil {
		return hex.EncodeToString(tx.Operation.Burn.From)
	} else if tx.Operation.Mint != nil {
		return ""
	} else if tx.Operation.Approve != nil {
		return hex.EncodeToString(tx.Operation.Approve.From)
	}

	return ""
}

func (tx Transaction) DestinationAddress() string {
	if tx.Operation.Transfer != nil {
		return hex.EncodeToString(tx.Operation.Transfer.To)
	} else if tx.Operation.Burn != nil {
		return ""
	} else if tx.Operation.Mint != nil {
		return hex.EncodeToString(tx.Operation.Mint.To)
	} else if tx.Operation.Approve != nil {
		return hex.EncodeToString(tx.Operation.Approve.Spender)
	}

	return ""
}

type Block struct {
	ParentHash  *[]byte     `ic:"parent_hash,omitempty" json:"parent_hash,omitempty"`
	Transaction Transaction `ic:"transaction" json:"transaction"`
	Timestamp   Timestamp   `ic:"timestamp" json:"timestamp"`
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

type ICRC1TransferArgs struct {
	FromSubaccount *[]byte      `ic:"from_subaccount,omitempty"`
	To             ICRC1Account `ic:"to"`
	Amount         *big.Int     `ic:"amount"`
	Fee            *big.Int     `ic:"fee,omitempty"`
	Memo           *[]byte      `ic:"memo,omitempty"`
	CreatedAtTime  *uint64      `ic:"created_at_time,omitempty"`
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
