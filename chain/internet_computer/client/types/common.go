package types

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sort"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/leb128"
	"github.com/fxamacker/cbor/v2"
)

type RequestType string

const (
	RequestTypeQuery = RequestType("query")
	RequestTypeCall  = RequestType("call")
)

var (
	typeKey          = sha256.Sum256([]byte("request_type"))
	canisterIDKey    = sha256.Sum256([]byte("canister_id"))
	nonceKey         = sha256.Sum256([]byte("nonce"))
	methodNameKey    = sha256.Sum256([]byte("method_name"))
	argumentsKey     = sha256.Sum256([]byte("arg"))
	ingressExpiryKey = sha256.Sum256([]byte("ingress_expiry"))
	senderKey        = sha256.Sum256([]byte("sender"))
	pathsKey         = sha256.Sum256([]byte("paths"))
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

func BadFeeError(expected uint64) string {
	return fmt.Sprintf("bad fee, expected: %d", expected)
}

func InsufficientFundsError(balance uint64) string {
	return fmt.Sprintf("insufficient funds, balance: %d", balance)
}

func TooOldError() string {
	return "transaction too old"
}

func CreatedInFutureError() string {
	return "transaction created in the future"
}

func UnknownError() string {
	return "unknown error"
}

type TransactionWithId struct {
	Id          uint64
	Transaction Transaction
}

type Transaction interface {
	Hash() (string, error)
	TxTime() uint64
	SourceAddress() string
	DestinationAddress() string
	Memo() string
	Amount() (uint64, error)
	Fee() uint64
}

type Block interface {
	Hash() (string, error)
	Timestamp() (uint64, error)
	TxHash() (string, error)
	Transaction() (Transaction, error)
	// Sometimes for ICRC transactions the fee is included in the block object, not transaction
	Fee() uint64
}

// Return a custom transaction hash in "{blockHeight}.{ledgerCanister}" format
// This is the fastest and the most reliable way to fetch transaction id.
func GetTransacionHash(blockHeight uint64, ledgerCanister address.Principal) string {
	return fmt.Sprintf("%d.%s", blockHeight, ledgerCanister)
}
