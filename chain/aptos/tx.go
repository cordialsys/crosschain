package aptos

import (
	"encoding/hex"
	"errors"
	"fmt"

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
	tx_signatures      []*xc.SignatureResponse
	extraFeePayer      xc.Address
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
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	msg, err := tx.tx.GetSigningMessage()
	if err != nil {
		return nil, err
	}
	if tx.extraFeePayer != "" {
		return []*xc.SignatureRequest{
			// Here the order matters, because .AddSignatures assumes the first signature is the sender.
			xc.NewSignatureRequest([]byte(msg[:])),
			xc.NewSignatureRequest([]byte(tx.extraFeePayer)),
		}, nil
	} else {
		return []*xc.SignatureRequest{
			xc.NewSignatureRequest([]byte(msg[:])),
		}, nil
	}
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...*xc.SignatureResponse) error {
	msg, err := tx.tx.GetSigningMessage()
	if err != nil {
		return err
	}
	if len(signatures) != 1 {
		return errors.New("expecting 1 signature")
	}
	tx.tx_signing_message = msg
	tx.tx_signatures = signatures[:]
	serialized, err := tx.Serialize()
	if err != nil {
		return err
	}
	tx.tx_serialized = serialized

	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	for _, sig := range tx.tx_signatures {
		sigs = append(sigs, sig.Signature)
	}
	return sigs
}

func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.tx_signatures) == 0 || len(tx.tx_signing_message) == 0 {
		return []byte{}, errors.New("unable to serialize without first calling AddSignatures(...)")
	}
	if len(tx.Input.Pubkey) == 0 {
		return []byte{}, errors.New("unable to serialize without setting public key")
	}

	// assume first signature is the sender
	publickey, err := transactionbuilder.NewEd25519PublicKey(tx.tx_signatures[0].PublicKey)
	if err != nil {
		return []byte{}, err
	}
	signature, err := transactionbuilder.NewEd25519Signature(tx.tx_signatures[0].Signature)
	if err != nil {
		return []byte{}, err
	}
	authSender := transactionbuilder.TransactionAuthenticatorEd25519{
		PublicKey: *publickey,
		Signature: *signature,
	}

	if len(tx.tx_signatures) == 1 {
		signedTxn := transactionbuilder.SignedTransaction{
			Transaction:   &tx.tx,
			Authenticator: authSender,
		}

		data, err := lcs.Marshal(signedTxn)

		return data, err
	} else {
		signedTxn := transactionbuilder.TransactionAuthenticatorMultiAgent{
			Sender: authSender,
		}
		for _, sig := range tx.tx_signatures[1:] {
			signerAddr := [transactionbuilder.ADDRESS_LENGTH]byte{}
			decoded, err := DecodeHex(string(sig.Address))
			if err != nil {
				return nil, fmt.Errorf("failed to decode signer address: %v", err)
			}
			copy(signerAddr[:], decoded)

			publickey, err := transactionbuilder.NewEd25519PublicKey(sig.PublicKey)
			if err != nil {
				return []byte{}, err
			}
			signature, err := transactionbuilder.NewEd25519Signature(sig.Signature)
			if err != nil {
				return []byte{}, err
			}

			signedTxn.SecondarySignerAddresses = append(signedTxn.SecondarySignerAddresses, transactionbuilder.AccountAddress(signerAddr))
			signedTxn.SecondarySigners = append(signedTxn.SecondarySigners, transactionbuilder.AccountAuthenticatorEd25519{
				PublicKey: *publickey,
				Signature: *signature,
			})
		}
		data, err := lcs.Marshal(signedTxn)

		return data, err
	}
}
