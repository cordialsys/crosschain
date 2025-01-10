package tx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	// "time"

	xc "github.com/cordialsys/crosschain"
	"github.com/stellar/go/xdr"
)

func NetworkId(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}

// TimeBounds represents the time window during which a Stellar transaction is considered valid.
//
// MinTime and MaxTime represent Stellar timebounds - a window of time over which the Transaction will be
// considered valid. In general, almost all Transactions benefit from setting an upper timebound, because once submitted,
// the status of a pending Transaction may remain unresolved for a long time if the network is congested.
// With an upper timebound, the submitter has a guaranteed time at which the Transaction is known to have either
// succeeded or failed, and can then take appropriate action (e.g. resubmit or mark as resolved).
//
// Create a TimeBounds struct using NewTimeout()
type TimeBounds struct {
	MinTime  int64
	MaxTime  int64
	wasBuilt bool
}

func NewTimeout(timeout time.Duration) TimeBounds {
	return TimeBounds{0, time.Now().Add(timeout).Unix(), true}
}

func NewInfiniteTimeout() TimeBounds {
	return TimeBounds{0, int64(0), false}
}

// Operation represents the operation types of the Stellar network.
type Operation interface {
	BuildXDR() (xdr.Operation, error)
	FromXDR(xdrOp xdr.Operation) error
	Validate() error
	GetSourceAccount() string
}

// Preconditions is a container for all transaction preconditions.
type Preconditions struct {
	// Transaction is only valid during a certain time range (units are seconds).
	TimeBounds TimeBounds
	// Transaction is valid for ledger numbers n such that minLedger <= n
	MinLedgerSequence int64
}

func (prec Preconditions) BuildXDR() xdr.Preconditions {
	xdrCond := xdr.Preconditions{}
	xdrTimeBounds := xdr.TimeBounds{
		MinTime: xdr.TimePoint(prec.TimeBounds.MinTime),
		MaxTime: xdr.TimePoint(prec.TimeBounds.MaxTime),
	}

	xdrCond.Type = xdr.PreconditionTypePrecondTime
	xdrCond.TimeBounds = &xdrTimeBounds

	return xdrCond
}

type Tx struct {
	TxEnvelope        *xdr.TransactionEnvelope
	Signatures        []xc.TxSignature
	// Passphrase used to build transaction hash.
	// Passphrases are public and defined per network. More info:
	// https://developers.stellar.org/docs/learn/encyclopedia/network-configuration/network-passphrases
	NetworkPassphrase string
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	if tx.TxEnvelope == nil {
		return xc.TxHash("")
	}

	hash, err := HashEnvelope(tx.TxEnvelope, tx.NetworkPassphrase)
	if err != nil {
		return xc.TxHash("")
	}

	hex := hex.EncodeToString(hash)
	return xc.TxHash(hex)
}

func HashEnvelope(envelope *xdr.TransactionEnvelope, passphrase string) ([]byte, error) {
	var hash []byte
	var err error
	if envelope == nil {
		return hash, errors.New("transaction envelope is missing")
	}

	switch envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		hash, err = HashTransactionV1(envelope.V1.Tx, passphrase)
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		err = errors.New("XLM Transaction type 0 is unsupported")
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		err = errors.New("XLM FeeBump transaction is unsupported")
	default:
		err = errors.New("Invalid transaction type")
	}

	return hash, err
}

func HashTransactionV1(transaction xdr.Transaction, passphrase string) ([]byte, error) {
	taggedTx := xdr.TransactionSignaturePayloadTaggedTransaction{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		Tx:   &transaction,
	}

	if strings.TrimSpace(passphrase) == "" {
		return []byte{}, errors.New("empty network passphrase")
	}

	payload := xdr.TransactionSignaturePayload{
		NetworkId:         sha256.Sum256([]byte(passphrase)),
		TaggedTransaction: taggedTx,
	}

	var txBytes bytes.Buffer
	_, err := xdr.Marshal(&txBytes, payload)
	if err != nil {
		return []byte{}, fmt.Errorf("failed tx marshal: %w", err)
	}

	hash := sha256.Sum256(txBytes.Bytes())
	return hash[:], nil
}

func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	var hash []byte
	var err error

	hash, err = HashEnvelope(tx.TxEnvelope, tx.NetworkPassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to hash envelope: %w", err)
	}

	return []xc.TxDataToSign{hash}, err
}

// Create a new xdr.DecoratedSignature, which is a signature bundled with last
// 4 bytes of signer public key
func NewDecoratedSignature(signature xc.TxSignature, pub_key []byte) (xdr.DecoratedSignature, error) {
	pub_key_len := len(pub_key)
	if pub_key_len < 32 {
		return xdr.DecoratedSignature{}, fmt.Errorf("specified public key is too short: %v, expected len: 32", pub_key_len)
	}

	return xdr.DecoratedSignature{
		Signature: xdr.Signature(signature),
		Hint:      xdr.SignatureHint(pub_key[pub_key_len-4:]),
	}, nil
}

func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx == nil {
		return errors.New("invalid transaction")
	}

	if tx.TxEnvelope == nil {
		return errors.New("missing transaction envelope")
	}

	if tx.Signatures != nil {
		return fmt.Errorf("transaction already signed")
	}

	pubKey, ok := tx.TxEnvelope.SourceAccount().GetEd25519()
	if ok != true {
		return errors.New("failed to retrieve public key from source account")
	}

	xlmSignatures := make([]xdr.DecoratedSignature, len(signatures))
	for i, signature := range signatures {
		decoratedSig, err := NewDecoratedSignature(signature, pubKey[:])
		if err != nil {
			return fmt.Errorf("failed to create decorated signature: %w", err)
		}

		xlmSignatures[i] = decoratedSig
	}

	tx.Signatures = signatures
	tx.TxEnvelope.V1.Signatures = xlmSignatures

	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.Signatures
}

func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.TxEnvelope.Operations()) == 0 {
		return []byte{}, errors.New("missing transaction operations")
	}

	if len(tx.TxEnvelope.Signatures()) == 0 {
		return []byte{}, errors.New("missing transaction signatures")
	}

	var txBytes bytes.Buffer
	_, err := xdr.Marshal(&txBytes, tx.TxEnvelope)
	if err != nil {
		return nil, err
	}
	return txBytes.Bytes(), nil
}
