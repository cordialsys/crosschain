package tx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/stellar/go/xdr"
)

const (
	MainnetNetworkPassphrase string = "Public Global Stellar Network ; September 2015"
	TestnetNetworkPassphrase string = "Test SDF Network ; September 2015"
	FuturenetPassphrase      string = "Test SDF Future Network ; October 2022"
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
// succeeded or failed, and can then take appropriate action (e.g. to resubmit or mark as resolved).
//
// Create a TimeBounds struct using one of NewTimebounds(), NewTimeout(), or NewInfiniteTimeout().
type TimeBounds struct {
	MinTime  int64
	MaxTime  int64
	wasBuilt bool
}

func NewTimeout(timeout int64) TimeBounds {
	return TimeBounds{0, time.Now().UTC().Unix() + timeout, true}
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
	// Transaction is valid for ledger numbers n such that minLedger <= n <
	// maxLedger (if maxLedger == 0, then only minLedger is checked)
	MinLedgerSequence *int64
	// If nil, the transaction is only valid when sourceAccount's sequence
	// number "N" is seqNum - 1. Otherwise, valid when N satisfies minSeqNum <=
	// N < tx.seqNum.
	MinSequenceNumber *int64
}

func (prec Preconditions) AreValid() bool {
	return prec.TimeBounds.MaxTime != 0
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
	XLMTx      *xdr.TransactionEnvelope
	SignPubKey []byte
	Signatures []xc.TxSignature
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	hash, err := HashEnvelope(tx.XLMTx, MainnetNetworkPassphrase)
	if err != nil {
		return xc.TxHash("")
	}

	hex := hex.EncodeToString(hash)
	return xc.TxHash(hex)
}

// Hash envelope with network passphrase
func HashEnvelope(envelope *xdr.TransactionEnvelope, passphrase string) ([]byte, error) {
	var hash []byte
	var err error

	switch envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		hash, err = HashTransactionV1(envelope.V1.Tx, passphrase)
	// TODO: Do we need support for TransactionV0?
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		err = errors.New("not implemented")
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

	// Derive this from the RPC url/type
	passphrase := MainnetNetworkPassphrase

	hash, err = HashEnvelope(tx.XLMTx, passphrase)
	if err != nil {
		return []xc.TxDataToSign{}, fmt.Errorf("failed to hash envelope: %w", err)
	}

	return []xc.TxDataToSign{hash}, err
}

// Create a new xdr.DecoratedSignature, which is a signature bundled with last
// 4 bytes of signer public key
func NewDecoratedSignature(signature xc.TxSignature, pub_key []byte) (xdr.DecoratedSignature, error) {
	pub_key_len := len(pub_key)
	if pub_key_len < 32 {
		return xdr.DecoratedSignature{}, fmt.Errorf("specified public key is too short: %v, xpected len: 32", pub_key_len)
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

	if tx.Signatures != nil {
		return fmt.Errorf("transaction already signed")
	}

	xlmSignatures := make([]xdr.DecoratedSignature, len(signatures))
	for i, signature := range signatures {
		decoratedSig, err := NewDecoratedSignature(signature, tx.SignPubKey)
		if err != nil {
			return fmt.Errorf("failed to create decorated signature: %w", err)
		}

		xlmSignatures[i] = decoratedSig
	}

	tx.Signatures = signatures
	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.Signatures
}

func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.XLMTx.Operations()) == 0 {
		return []byte{}, errors.New("missing transaction operations")
	}

	if len(tx.XLMTx.Signatures()) == 0 {
		return []byte{}, errors.New("missing transaction signatures")
	}

	switch tx.XLMTx.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		tx.XLMTx.V1.Signatures = tx.XLMTx.Signatures()
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
	default:
		return []byte{}, errors.New("not implemented")
	}

	var txBytes bytes.Buffer
	_, err := xdr.Marshal(&txBytes, tx.XLMTx)
	if err != nil {
		return []byte{}, errors.New("failed to marshal transaction envelope")
	}

	return txBytes.Bytes(), nil
}
