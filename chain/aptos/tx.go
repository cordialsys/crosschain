package aptos

import (
	"encoding/hex"
	"errors"

	transactionbuilder "github.com/coming-chat/go-aptos/transaction_builder"
	"github.com/coming-chat/lcs"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
	"golang.org/x/crypto/sha3"
)

type Tx struct {
	Input              *tx_input.TxInput
	tx                 transactionbuilder.RawTransaction
	tx_serialized      []byte
	tx_signing_message []byte
	tx_signature       []byte
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	// Prefix with the type
	TRANSACTION_SALT := []byte("APTOS::Transaction")
	prefix := sha3.Sum256(TRANSACTION_SALT)
	// Must prefix with 0
	hash_base := append(prefix[:], 0)
	// Hash over serialized signed transaction
	hash_base = append(hash_base, tx.tx_serialized...)
	hash := sha3.Sum256(hash_base)
	return xc.TxHash(hex.EncodeToString(hash[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	msg, err := tx.tx.GetSigningMessage()
	return []xc.TxDataToSign{[]byte(msg[:])}, err
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	msg, err := tx.tx.GetSigningMessage()
	if err != nil {
		return err
	}
	if len(signatures) != 1 {
		return errors.New("expecting 1 signature")
	}
	tx.tx_signing_message = msg
	tx.tx_signature = signatures[0]
	serialized, err := tx.Serialize()
	if err != nil {
		return err
	}
	tx.tx_serialized = serialized

	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	if len(tx.tx_signature) > 0 {
		sigs = append(sigs, tx.tx_signature)
	}
	return sigs
}

func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.tx_signature) == 0 || len(tx.tx_signing_message) == 0 {
		return []byte{}, errors.New("unable to serialize without first calling AddSignatures(...)")
	}
	if len(tx.Input.Pubkey) == 0 {
		return []byte{}, errors.New("unable to serialize without setting public key")
	}

	publickey, err := transactionbuilder.NewEd25519PublicKey(tx.Input.Pubkey)
	if err != nil {
		return []byte{}, err
	}
	signature, err := transactionbuilder.NewEd25519Signature(tx.tx_signature)
	if err != nil {
		return []byte{}, err
	}
	authenticator := transactionbuilder.TransactionAuthenticatorEd25519{
		PublicKey: *publickey,
		Signature: *signature,
	}
	signedTxn := transactionbuilder.SignedTransaction{
		Transaction:   &tx.tx,
		Authenticator: authenticator,
	}

	data, err := lcs.Marshal(signedTxn)
	return data, err
}
