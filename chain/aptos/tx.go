package aptos

import (
	"encoding/hex"
	"errors"

	transactionbuilder "github.com/coming-chat/go-aptos/transaction_builder"
	"github.com/coming-chat/lcs"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

type Tx struct {
	Input *tx_input.TxInput
	rawTx transactionbuilder.RawTransaction
	// tx_serialized []byte
	txSignatures  []*xc.SignatureResponse
	extraFeePayer xc.Address
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
	serialized, err := tx.Serialize()
	if err != nil {
		logrus.WithError(err).Error("failed to serialize tx for hash")
		return xc.TxHash("")
	}
	hash_base = append(hash_base, serialized...)
	hash := sha3.Sum256(hash_base)
	return xc.TxHash(hex.EncodeToString(hash[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if tx.extraFeePayer != "" {
		feePayerAddr, err := DecodeAddress(string(tx.extraFeePayer))
		if err != nil {
			return nil, err
		}
		// Confusingly, this is _not_ the exact message that gets submitted/broadcast.
		// Instead APTOS requires this convention of signing a slightly different message
		// for fee-payer transactions.
		// https://aptos.dev/en/build/guides/sponsored-transactions
		msg, err := getFeePayerSigningMessage(tx.rawTx, nil, &feePayerAddr)
		if err != nil {
			return nil, err
		}

		return []*xc.SignatureRequest{
			// Here the order matters, because .AddSignatures assumes the first signature is the sender.
			xc.NewSignatureRequest([]byte(msg)),
			xc.NewSignatureRequest([]byte(msg), tx.extraFeePayer),
		}, nil
	} else {
		msg, err := getRawTransactionSigningMessage(tx.rawTx)
		if err != nil {
			return nil, err
		}
		return []*xc.SignatureRequest{
			xc.NewSignatureRequest([]byte(msg[:])),
		}, nil
	}
}

func getRawTransactionSigningMessage(
	rawTx transactionbuilder.RawTransaction,
) (transactionbuilder.SigningMessage, error) {
	msg, err := rawTx.GetSigningMessage()
	return msg, err
}

func getFeePayerSigningMessage(
	rawTx transactionbuilder.RawTransaction,
	secondarySignerAddresses []transactionbuilder.AccountAddress,
	feePayerAddressMaybe *[transactionbuilder.ADDRESS_LENGTH]byte,

) (transactionbuilder.SigningMessage, error) {
	feePayerAddress := transactionbuilder.AccountAddress{}
	if feePayerAddressMaybe != nil {
		feePayerAddress = *feePayerAddressMaybe
	}
	feePayerTx := transactionbuilder.MultiAgentRawTransactionWithFeePayer{
		RawTransaction: rawTx,
		// no secondary signers
		SecondarySignerAddresses: secondarySignerAddresses,
		FeePayerAddress:          transactionbuilder.AccountAddress(feePayerAddress),
	}
	msg, err := feePayerTx.GetSigningMessage()
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) == 0 {
		return errors.New("expecting >=1 signature")
	}
	for _, sig := range signatures {
		if sig.Address == "" {
			return errors.New("address for signature is required")
		}
		if len(sig.PublicKey) == 0 {
			return errors.New("public key for signature is required")
		}
	}
	tx.txSignatures = signatures[:]
	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	for _, sig := range tx.txSignatures {
		sigs = append(sigs, sig.Signature)
	}
	return sigs
}

func (tx Tx) Serialize() ([]byte, error) {
	if len(tx.txSignatures) == 0 {
		return []byte{}, errors.New("unable to serialize without first calling AddSignatures(...)")
	}

	// assume first signature is the sender
	publickey, err := transactionbuilder.NewEd25519PublicKey(tx.txSignatures[0].PublicKey)
	if err != nil {
		return []byte{}, err
	}
	signature, err := transactionbuilder.NewEd25519Signature(tx.txSignatures[0].Signature)
	if err != nil {
		return []byte{}, err
	}
	authSender := transactionbuilder.TransactionAuthenticatorEd25519{
		PublicKey: *publickey,
		Signature: *signature,
	}
	signedTxn := transactionbuilder.SignedTransaction{
		Transaction:   &tx.rawTx,
		Authenticator: authSender,
	}
	var data []byte
	if len(tx.txSignatures) > 1 {
		// https://aptos.dev/en/build/guides/sponsored-transactions
		feePayerSig := tx.txSignatures[1]
		feePayerAddr, err := DecodeAddress(string(feePayerSig.Address))
		if err != nil {
			return nil, err
		}
		feePayerPublicKey, err := transactionbuilder.NewEd25519PublicKey(feePayerSig.PublicKey)
		if err != nil {
			return nil, err
		}
		feePayerSignature, err := transactionbuilder.NewEd25519Signature(feePayerSig.Signature)
		if err != nil {
			return nil, err
		}
		authenticator := transactionbuilder.TransactionAuthenticatorFeePayer{
			Sender:                   transactionbuilder.AccountAuthenticatorEd25519(authSender),
			SecondarySignerAddresses: []transactionbuilder.AccountAddress{},
			SecondarySigners:         []transactionbuilder.AccountAuthenticator{},
			FeePayerAddress:          transactionbuilder.AccountAddress(feePayerAddr),
			FeePayerSigner: transactionbuilder.AccountAuthenticatorEd25519{
				PublicKey: *feePayerPublicKey,
				Signature: *feePayerSignature,
			},
		}
		signedTxn.Authenticator = authenticator
	}
	data, err = lcs.Marshal(&signedTxn)
	if err != nil {
		return nil, err
	}

	return data, err
}
