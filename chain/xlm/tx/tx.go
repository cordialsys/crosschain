package tx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/stellar/go/xdr"
)

type Tx struct {
	TxEnvelope *xdr.TransactionEnvelope
	Signatures []xc.TxSignature
	// NetworkPassphrase is used to build the transaction hash.
	// Passphrases are publicly defined for each network.
	// For more information, see the Stellar documentation:
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
	if envelope == nil {
		return []byte{}, errors.New("transaction envelope is missing")
	}

	switch envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		return HashTransactionV1(envelope.V1.Tx, passphrase)
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		return []byte{}, errors.New("XLM Transaction type 0 is unsupported")
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		return []byte{}, errors.New("XLM FeeBump transaction is unsupported")
	default:
		return []byte{}, errors.New("Invalid transaction type")
	}
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

func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	var hash []byte
	var err error

	hash, err = HashEnvelope(tx.TxEnvelope, tx.NetworkPassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to hash envelope: %w", err)
	}

	return []*xc.SignatureRequest{xc.NewSignatureRequest(hash)}, err
}

// NewDecoratedSignature creates a new xdr.DecoratedSignature, which combines a signature
// with the last 4 bytes of the signer's public key (called Hint).
func NewDecoratedSignature(signature xc.TxSignature, pub_key []byte) (xdr.DecoratedSignature, error) {
	pub_key_len := len(pub_key)
	if pub_key_len != 32 {
		return xdr.DecoratedSignature{}, fmt.Errorf("specified public key is invalid: %v, expected len: 32", pub_key_len)
	}

	return xdr.DecoratedSignature{
		Signature: xdr.Signature(signature),
		Hint:      xdr.SignatureHint(pub_key[pub_key_len-4:]),
	}, nil
}

func (tx *Tx) AddSignatures(signatures ...*xc.SignatureResponse) error {
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
	if !ok {
		return errors.New("failed to retrieve public key from source account")
	}

	xlmSignatures := make([]xdr.DecoratedSignature, len(signatures))
	for i, signature := range signatures {
		decoratedSig, err := NewDecoratedSignature(signature.Signature, pubKey[:])
		if err != nil {
			return fmt.Errorf("failed to create decorated signature: %w", err)
		}

		xlmSignatures[i] = decoratedSig
	}

	tx.Signatures = make([]xc.TxSignature, len(signatures))
	for i, sig := range signatures {
		tx.Signatures[i] = sig.Signature
	}
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
