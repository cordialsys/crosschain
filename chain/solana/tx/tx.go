package tx

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Tx for Solana, encapsulating a solana.Transaction and other info
type Tx struct {
	SolTx            *solana.Transaction
	inputSignatures  []xc.TxSignature
	transientSigners []solana.PrivateKey
	extraFeePayer    xc.Address
}

var _ xc.Tx = &Tx{}

func (tx *Tx) SetExtraFeePayerSigner(extraFeePayer xc.Address) {
	tx.extraFeePayer = extraFeePayer
}

// Hash returns the tx hash or id, for Solana it's signature
func (tx Tx) Hash() xc.TxHash {
	if tx.SolTx != nil && len(tx.SolTx.Signatures) > 0 {
		sig := tx.SolTx.Signatures[0]
		return xc.TxHash(sig.String())
	}
	return xc.TxHash("")
}

// Sighashes returns the tx payload to sign, aka sighashes
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if tx.SolTx == nil {
		return nil, errors.New("transaction not initialized")
	}
	messageContent, err := tx.SolTx.Message.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to encode message for signing: %w", err)
	}
	if tx.extraFeePayer != "" {
		return []*xc.SignatureRequest{
			// first signature from extra fee payer
			xc.NewSignatureRequest(messageContent, tx.extraFeePayer),
			// then the main address signature
			xc.NewSignatureRequest(messageContent),
		}, nil
	} else {
		// single signature from main address
		return []*xc.SignatureRequest{
			xc.NewSignatureRequest(messageContent),
		}, nil
	}
}

// Some instructions on solana require new accounts to sign the transaction
// in addition to the funding account.  These are transient signers are not
// sensitive and the key material only needs to live long enough to sign the transaction.
func (tx *Tx) AddTransientSigner(transientSigner solana.PrivateKey) {
	tx.transientSigners = append(tx.transientSigners, transientSigner)
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if tx.SolTx == nil {
		return errors.New("transaction not initialized")
	}
	tx.inputSignatures = []xc.TxSignature{}
	solSignatures := make([]solana.Signature, len(signatures))
	for i, signature := range signatures {
		if len(signature.Signature) != solana.SignatureLength {
			return fmt.Errorf("invalid signature (%d): %x", len(signature.Signature), signature.Signature)
		}
		copy(solSignatures[i][:], signature.Signature)
		tx.inputSignatures = append(tx.inputSignatures, xc.TxSignature(signature.Signature))
	}
	tx.SolTx.Signatures = solSignatures

	// add transient signers
	for _, transient := range tx.transientSigners {
		bz, _ := tx.SolTx.Message.MarshalBinary()
		sig, err := transient.Sign(bz)
		if err != nil {
			return fmt.Errorf("unable to sign with transient signer: %v", err)
		}
		tx.SolTx.Signatures = append(tx.SolTx.Signatures, sig)
		tx.inputSignatures = append(tx.inputSignatures, xc.TxSignature(sig[:]))
	}
	return nil
}

func NewTxFrom(solTx *solana.Transaction) *Tx {
	tx := &Tx{
		SolTx: solTx,
	}
	return tx
}

// RecentBlockhash returns the recent block hash used as a nonce for a Solana tx
func (tx Tx) RecentBlockhash() string {
	if tx.SolTx != nil {
		return tx.SolTx.Message.RecentBlockhash.String()
	}
	return ""
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	if tx.SolTx == nil {
		return []byte{}, errors.New("transaction not initialized")
	}
	return tx.SolTx.MarshalBinary()
}

func (tx Tx) GetDecoder() (*Decoder, error) {
	if tx.SolTx == nil {
		return nil, errors.New("transaction not initialized")
	}
	return NewDecoderFromNativeTx(tx.SolTx, &rpc.TransactionMeta{}), nil
}
